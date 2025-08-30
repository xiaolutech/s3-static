#!/bin/bash

# Test coverage script for S3-Static project
set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
COVERAGE_THRESHOLD=80
COVERAGE_FILE="coverage.out"
COVERAGE_HTML="coverage.html"
COVERAGE_JSON="coverage.json"

echo -e "${BLUE}开始全面的测试覆盖率分析...${NC}"

# Create coverage directory if it doesn't exist
mkdir -p coverage

# Function to print colored output
print_status() {
    echo -e "${GREEN}[信息]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[警告]${NC} $1"
}

print_error() {
    echo -e "${RED}[错误]${NC} $1"
}

# Clean previous coverage data
print_status "清理之前的覆盖率数据..."
rm -f ${COVERAGE_FILE} ${COVERAGE_HTML} ${COVERAGE_JSON}
go clean -testcache

# Run unit tests with coverage
print_status "运行单元测试并生成覆盖率..."
go test -v -race -coverprofile=${COVERAGE_FILE} -covermode=atomic ./internal/... ./pkg/... . 2>&1 | tee coverage/unit-test.log

# Check if coverage file was generated
if [ ! -f ${COVERAGE_FILE} ]; then
    print_error "Coverage file not generated. Tests may have failed."
    exit 1
fi

# Generate coverage report
print_status "Generating coverage reports..."

# HTML report
go tool cover -html=${COVERAGE_FILE} -o ${COVERAGE_HTML}
print_status "HTML coverage report generated: ${COVERAGE_HTML}"

# Function-level coverage
go tool cover -func=${COVERAGE_FILE} > coverage/coverage-func.txt
print_status "Function-level coverage saved to coverage/coverage-func.txt"

# Extract overall coverage percentage
COVERAGE_PERCENT=$(go tool cover -func=${COVERAGE_FILE} | grep total | awk '{print $3}' | sed 's/%//')

echo ""
echo -e "${BLUE}=== COVERAGE SUMMARY ===${NC}"
echo -e "Overall Coverage: ${GREEN}${COVERAGE_PERCENT}%${NC}"

# Check coverage threshold
if (( $(echo "$COVERAGE_PERCENT >= $COVERAGE_THRESHOLD" | bc -l) )); then
    echo -e "Coverage Status: ${GREEN}PASS${NC} (>= ${COVERAGE_THRESHOLD}%)"
else
    echo -e "Coverage Status: ${RED}FAIL${NC} (< ${COVERAGE_THRESHOLD}%)"
    print_warning "Coverage is below threshold of ${COVERAGE_THRESHOLD}%"
fi

echo ""

# Package-level coverage breakdown
echo -e "${BLUE}=== PACKAGE COVERAGE BREAKDOWN ===${NC}"
go tool cover -func=${COVERAGE_FILE} | grep -E "^[^[:space:]]" | grep -v "total:" | while read line; do
    package=$(echo $line | awk '{print $1}' | sed 's/.*\///')
    coverage=$(echo $line | awk '{print $3}')
    echo -e "  ${package}: ${coverage}"
done

echo ""

# Find uncovered functions
echo -e "${BLUE}=== UNCOVERED FUNCTIONS ===${NC}"
go tool cover -func=${COVERAGE_FILE} | grep "0.0%" | while read line; do
    func_name=$(echo $line | awk '{print $2}')
    echo -e "  ${RED}${func_name}${NC}"
done

# Generate detailed coverage analysis
print_status "Generating detailed coverage analysis..."

# Create a detailed report
cat > coverage/detailed-report.md << EOF
# Test Coverage Report

Generated on: $(date)

## Summary
- **Overall Coverage:** ${COVERAGE_PERCENT}%
- **Threshold:** ${COVERAGE_THRESHOLD}%
- **Status:** $(if (( $(echo "$COVERAGE_PERCENT >= $COVERAGE_THRESHOLD" | bc -l) )); then echo "PASS"; else echo "FAIL"; fi)

## Package Breakdown

EOF

# Add package breakdown to markdown report
go tool cover -func=${COVERAGE_FILE} | grep -E "^[^[:space:]]" | grep -v "total:" | while read line; do
    package=$(echo $line | awk '{print $1}' | sed 's/.*\///')
    coverage=$(echo $line | awk '{print $3}')
    echo "- **${package}:** ${coverage}" >> coverage/detailed-report.md
done

cat >> coverage/detailed-report.md << EOF

## Uncovered Functions

EOF

# Add uncovered functions to markdown report
uncovered_count=0
go tool cover -func=${COVERAGE_FILE} | grep "0.0%" | while read line; do
    func_name=$(echo $line | awk '{print $2}')
    echo "- \`${func_name}\`" >> coverage/detailed-report.md
    uncovered_count=$((uncovered_count + 1))
done

if [ $uncovered_count -eq 0 ]; then
    echo "No uncovered functions found! 🎉" >> coverage/detailed-report.md
fi

cat >> coverage/detailed-report.md << EOF

## Files

View the [HTML coverage report](../coverage.html) for detailed line-by-line coverage.

## Recommendations

EOF

# Add recommendations based on coverage
if (( $(echo "$COVERAGE_PERCENT < 70" | bc -l) )); then
    cat >> coverage/detailed-report.md << EOF
- ⚠️  Coverage is below 70%. Consider adding more unit tests.
- Focus on testing error paths and edge cases.
- Add integration tests for critical workflows.
EOF
elif (( $(echo "$COVERAGE_PERCENT < 85" | bc -l) )); then
    cat >> coverage/detailed-report.md << EOF
- 📈 Good coverage! Consider adding tests for remaining uncovered functions.
- Focus on testing error handling and edge cases.
EOF
else
    cat >> coverage/detailed-report.md << EOF
- ✅ Excellent coverage! Maintain this level with new code.
- Consider adding property-based tests for complex functions.
EOF
fi

print_status "Detailed report saved to coverage/detailed-report.md"

# Run benchmark tests if requested
if [ "$1" = "--with-benchmarks" ]; then
    print_status "Running benchmark tests..."
    go test -bench=. -benchmem . > coverage/benchmark-results.txt 2>&1
    print_status "Benchmark results saved to coverage/benchmark-results.txt"
fi

# Generate test statistics
print_status "Generating test statistics..."
cat > coverage/test-stats.txt << EOF
Test Statistics
===============

Generated: $(date)

Unit Test Results:
$(grep -E "(PASS|FAIL|RUN)" coverage/unit-test.log | tail -20)

Coverage Summary:
- Overall: ${COVERAGE_PERCENT}%
- Threshold: ${COVERAGE_THRESHOLD}%
- Files Analyzed: $(go tool cover -func=${COVERAGE_FILE} | grep -c "\.go:")
- Total Functions: $(go tool cover -func=${COVERAGE_FILE} | grep -v "total:" | wc -l)
- Uncovered Functions: $(go tool cover -func=${COVERAGE_FILE} | grep -c "0.0%")

EOF

# Check for race conditions in tests
print_status "Checking for race conditions..."
if go test -race ./... > coverage/race-test.log 2>&1; then
    echo "✅ No race conditions detected" >> coverage/test-stats.txt
else
    echo "⚠️  Race conditions detected - check race-test.log" >> coverage/test-stats.txt
    print_warning "Race conditions detected. Check coverage/race-test.log"
fi

# Final summary
echo ""
echo -e "${BLUE}=== FINAL SUMMARY ===${NC}"
echo -e "Coverage File: ${COVERAGE_FILE}"
echo -e "HTML Report: ${COVERAGE_HTML}"
echo -e "Detailed Report: coverage/detailed-report.md"
echo -e "Test Statistics: coverage/test-stats.txt"

if (( $(echo "$COVERAGE_PERCENT >= $COVERAGE_THRESHOLD" | bc -l) )); then
    echo -e "Result: ${GREEN}SUCCESS${NC} ✅"
    exit 0
else
    echo -e "Result: ${RED}NEEDS IMPROVEMENT${NC} ❌"
    echo -e "Please add more tests to reach ${COVERAGE_THRESHOLD}% coverage."
    exit 1
fi