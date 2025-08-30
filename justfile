# justfile for S3-Static project

# 默认显示帮助信息
default:
    @just --list

# 运行所有测试
test: test-unit test-integration test-examples-short

# 运行单元测试
test-unit:
    @echo "运行单元测试..."
    go test -v ./internal/... ./pkg/...

# 运行集成测试
test-integration:
    @echo "运行集成测试..."
    go test -v -tags=integration .

# 运行基准测试
test-benchmark:
    @echo "运行基准测试..."
    go test -bench=. -benchmem .

# 运行测试并生成覆盖率报告
test-coverage:
    @echo "运行测试并生成覆盖率报告..."
    go test -v -coverprofile=coverage.out ./internal/... ./pkg/... ./examples/... .
    go tool cover -html=coverage.out -o coverage.html
    @echo "覆盖率报告已生成: coverage.html"

# 运行详细的测试（包含竞态检测）
test-verbose:
    @echo "运行详细测试（包含竞态检测）..."
    go test -v -race ./internal/... ./pkg/... .

# 运行竞态检测
test-race:
    @echo "运行竞态检测..."
    go test -race ./internal/... ./pkg/... .

# 生成覆盖率报告
coverage-report: test-coverage
    go tool cover -func=coverage.out

# 在浏览器中打开覆盖率报告
coverage-html: test-coverage
    @echo "在浏览器中打开覆盖率报告..."
    @if command -v open >/dev/null 2>&1; then \
        open coverage.html; \
    elif command -v xdg-open >/dev/null 2>&1; then \
        xdg-open coverage.html; \
    elif command -v start >/dev/null 2>&1; then \
        start coverage.html; \
    else \
        echo "请手动打开 coverage.html"; \
    fi

# 构建应用程序
build:
    @echo "构建应用程序..."
    go build -o s3-static ./cmd/s3-static

# 为 Linux 构建
build-linux:
    @echo "为 Linux 构建..."
    GOOS=linux GOARCH=amd64 go build -o s3-static-linux ./cmd/s3-static

# 为 Windows 构建
build-windows:
    @echo "为 Windows 构建..."
    GOOS=windows GOARCH=amd64 go build -o s3-static.exe ./cmd/s3-static

# 为 macOS 构建
build-darwin:
    @echo "为 macOS 构建..."
    GOOS=darwin GOARCH=amd64 go build -o s3-static-darwin ./cmd/s3-static

# 构建所有平台
build-all: build-linux build-windows build-darwin

# 运行应用程序
run:
    @echo "运行应用程序..."
    go run ./cmd/s3-static

# 开发模式运行
run-dev:
    @echo "开发模式运行..."
    LOG_LEVEL=debug go run ./cmd/s3-static

# 运行 S3 使用示例
run-s3-example:
    @echo "运行 S3 使用示例..."
    go run ./examples/s3-usage/

# 运行 MinIO 头部演示
run-minio-example:
    @echo "运行 MinIO 头部演示..."
    go run ./examples/minio-demo/

# 运行示例测试
test-examples:
    @echo "运行示例测试..."
    go test -v ./examples/...

# 运行示例测试（短模式）
test-examples-short:
    @echo "运行示例测试（短模式）..."
    go test -v -short ./examples/...

# 格式化代码
fmt:
    @echo "格式化代码..."
    go fmt ./...

# 运行 linter
lint:
    @echo "运行 linter..."
    golangci-lint run

# 运行 go vet
vet:
    @echo "运行 go vet..."
    go vet ./...

# 整理 go modules
mod-tidy:
    @echo "整理 go modules..."
    go mod tidy

# 下载依赖
mod-download:
    @echo "下载依赖..."
    go mod download

# 构建 Docker 镜像
docker-build:
    @echo "构建 Docker 镜像..."
    docker build -t s3-static .

# 运行 Docker 容器
docker-run:
    @echo "运行 Docker 容器..."
    docker run -p 8080:8080 s3-static

# 清理构建产物和测试缓存
clean:
    @echo "清理..."
    go clean -testcache
    rm -f coverage.out coverage.html
    rm -f s3-static s3-static-* *.exe

# 深度清理
clean-all: clean
    @echo "深度清理..."
    go clean -modcache
    docker system prune -f

# CPU 性能分析
profile-cpu:
    @echo "运行 CPU 性能分析..."
    go test -cpuprofile=cpu.prof -bench=. .
    go tool pprof cpu.prof

# 内存性能分析
profile-mem:
    @echo "运行内存性能分析..."
    go test -memprofile=mem.prof -bench=. .
    go tool pprof mem.prof

# CI 测试
ci-test:
    @echo "运行 CI 测试..."
    go test -v -race -coverprofile=coverage.out ./internal/... ./pkg/... .
    go tool cover -func=coverage.out

# CI linting
ci-lint:
    @echo "运行 CI linting..."
    golangci-lint run --out-format=github-actions

# 开发环境设置
setup:
    @echo "设置开发环境..."
    go mod download
    @if ! command -v golangci-lint >/dev/null 2>&1; then \
        echo "安装 golangci-lint..."; \
        go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest; \
    fi

# 安全扫描
security:
    @echo "运行安全扫描..."
    @if command -v gosec >/dev/null 2>&1; then \
        gosec ./...; \
    else \
        echo "gosec 未安装，请运行 'just install-tools' 安装"; \
    fi

# 检查依赖更新
deps-check:
    @echo "检查依赖更新..."
    go list -u -m all

# 生成代码（如果使用 mockgen）
generate:
    @echo "生成代码..."
    go generate ./...

# 安装开发工具
install-tools:
    @echo "安装开发工具..."
    go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
    go install github.com/securecodewarrior/gosec/v2/cmd/gosec@latest
    go install golang.org/x/tools/cmd/goimports@latest

# 快速开发周期
dev: fmt vet test-unit

# 完整验证（用于 CI）
validate: fmt vet lint test ci-test

# 性能测试
perf: test-benchmark profile-cpu profile-mem

# 运行覆盖率脚本
coverage-script:
    @echo "运行覆盖率分析脚本..."
    @if [ -f scripts/test-coverage.sh ]; then \
        chmod +x scripts/test-coverage.sh && ./scripts/test-coverage.sh; \
    else \
        echo "覆盖率脚本不存在"; \
    fi

# 运行覆盖率脚本（包含基准测试）
coverage-script-with-benchmarks:
    @echo "运行覆盖率分析脚本（包含基准测试）..."
    @if [ -f scripts/test-coverage.sh ]; then \
        chmod +x scripts/test-coverage.sh && ./scripts/test-coverage.sh --with-benchmarks; \
    else \
        echo "覆盖率脚本不存在"; \
    fi

# 检查代码质量
quality: fmt vet lint test-race

# 预提交检查
pre-commit: quality test-coverage

# 发布准备
release-prep: clean build-all test-coverage validate

# 显示项目信息
info:
    @echo "S3-Static 项目信息"
    @echo "=================="
    @echo "Go 版本: $(go version)"
    @echo "项目路径: $(pwd)"
    @echo "Git 分支: $(git branch --show-current 2>/dev/null || echo 'N/A')"
    @echo "Git 提交: $(git rev-parse --short HEAD 2>/dev/null || echo 'N/A')"
    @echo ""
    @echo "可用命令:"
    @just --list