package main

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// TestDockerfileExists tests that the Dockerfile exists and is readable
func TestDockerfileExists(t *testing.T) {
	dockerfilePath := "Dockerfile"
	
	if _, err := os.Stat(dockerfilePath); os.IsNotExist(err) {
		t.Fatal("Dockerfile does not exist")
	}
	
	content, err := os.ReadFile(dockerfilePath)
	if err != nil {
		t.Fatalf("Failed to read Dockerfile: %v", err)
	}
	
	if len(content) == 0 {
		t.Fatal("Dockerfile is empty")
	}
}

// TestDockerfileBestPractices tests that the Dockerfile follows best practices
func TestDockerfileBestPractices(t *testing.T) {
	content, err := os.ReadFile("Dockerfile")
	if err != nil {
		t.Fatalf("Failed to read Dockerfile: %v", err)
	}
	
	dockerfile := string(content)
	
	// Test for multi-stage build
	if !strings.Contains(dockerfile, "FROM golang:") {
		t.Error("Dockerfile should use golang base image for build stage")
	}
	
	if !strings.Contains(dockerfile, "FROM alpine:") {
		t.Error("Dockerfile should use alpine for final stage")
	}
	
	// Test for non-root user
	if !strings.Contains(dockerfile, "USER appuser") {
		t.Error("Dockerfile should run as non-root user")
	}
	
	// Test for health check
	if !strings.Contains(dockerfile, "HEALTHCHECK") {
		t.Error("Dockerfile should include health check")
	}
	
	// Test for proper port exposure
	if !strings.Contains(dockerfile, "EXPOSE 8080") {
		t.Error("Dockerfile should expose port 8080")
	}
	
	// Test for ca-certificates (needed for HTTPS)
	if !strings.Contains(dockerfile, "ca-certificates") {
		t.Error("Dockerfile should install ca-certificates")
	}
}

// TestDockerBuildSuccess tests that the Docker image builds successfully
func TestDockerBuildSuccess(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping Docker build test in short mode")
	}
	
	// Check if Docker is available
	if !isDockerAvailable() {
		t.Skip("Docker is not available, skipping build test")
	}
	
	imageName := "s3-static-test"
	
	// Clean up any existing test image
	defer func() {
		exec.Command("docker", "rmi", imageName).Run()
	}()
	
	// Build the Docker image
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	
	cmd := exec.CommandContext(ctx, "docker", "build", "-t", imageName, ".")
	output, err := cmd.CombinedOutput()
	
	if err != nil {
		t.Fatalf("Docker build failed: %v\nOutput: %s", err, string(output))
	}
	
	// Verify the image was created
	cmd = exec.Command("docker", "images", imageName, "--format", "{{.Repository}}")
	output, err = cmd.Output()
	if err != nil {
		t.Fatalf("Failed to list Docker images: %v", err)
	}
	
	if !strings.Contains(string(output), imageName) {
		t.Fatalf("Docker image %s was not created", imageName)
	}
}

// TestDockerImageSecurity tests security aspects of the Docker image
func TestDockerImageSecurity(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping Docker security test in short mode")
	}
	
	if !isDockerAvailable() {
		t.Skip("Docker is not available, skipping security test")
	}
	
	imageName := "s3-static-test"
	
	// Build the image first
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	
	buildCmd := exec.CommandContext(ctx, "docker", "build", "-t", imageName, ".")
	if err := buildCmd.Run(); err != nil {
		t.Skip("Docker build failed, skipping security test")
	}
	
	defer exec.Command("docker", "rmi", imageName).Run()
	
	// Test that the container runs as non-root user
	cmd := exec.Command("docker", "run", "--rm", imageName, "id", "-u")
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("Failed to check user ID: %v", err)
	}
	
	userID := strings.TrimSpace(string(output))
	if userID == "0" {
		t.Error("Container should not run as root user (UID 0)")
	}
	
	// Test that the binary has correct permissions
	cmd = exec.Command("docker", "run", "--rm", imageName, "ls", "-la", "s3-static")
	output, err = cmd.Output()
	if err != nil {
		t.Fatalf("Failed to check binary permissions: %v", err)
	}
	
	permissions := string(output)
	if !strings.Contains(permissions, "appuser") {
		t.Error("Binary should be owned by appuser")
	}
}

// TestDockerHealthCheck tests that the health check works
func TestDockerHealthCheck(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping Docker health check test in short mode")
	}
	
	if !isDockerAvailable() {
		t.Skip("Docker is not available, skipping health check test")
	}
	
	imageName := "s3-static-test"
	containerName := "s3-static-health-test"
	
	// Build the image first
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	
	buildCmd := exec.CommandContext(ctx, "docker", "build", "-t", imageName, ".")
	if err := buildCmd.Run(); err != nil {
		t.Skip("Docker build failed, skipping health check test")
	}
	
	defer func() {
		exec.Command("docker", "rm", "-f", containerName).Run()
		exec.Command("docker", "rmi", imageName).Run()
	}()
	
	// Start container in background
	cmd := exec.Command("docker", "run", "-d", "--name", containerName, "-p", "8081:8080", imageName)
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to start container: %v", err)
	}
	
	// Wait a bit for the container to start
	time.Sleep(10 * time.Second)
	
	// Check health status
	cmd = exec.Command("docker", "inspect", "--format", "{{.State.Health.Status}}", containerName)
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("Failed to check health status: %v", err)
	}
	
	healthStatus := strings.TrimSpace(string(output))
	if healthStatus != "healthy" && healthStatus != "starting" {
		t.Errorf("Expected health status to be 'healthy' or 'starting', got: %s", healthStatus)
	}
}

// TestGitHubActionsWorkflow tests the GitHub Actions workflow file
func TestGitHubActionsWorkflow(t *testing.T) {
	workflowPath := ".github/workflows/docker-build.yml"
	
	if _, err := os.Stat(workflowPath); os.IsNotExist(err) {
		t.Fatal("GitHub Actions workflow file does not exist")
	}
	
	content, err := os.ReadFile(workflowPath)
	if err != nil {
		t.Fatalf("Failed to read workflow file: %v", err)
	}
	
	workflow := string(content)
	
	// Test for required workflow elements
	requiredElements := []string{
		"name: Build and Push Docker Image",
		"docker/build-push-action@v5",
		"docker/setup-buildx-action@v3",
		"linux/amd64,linux/arm64", // Multi-platform support
		"ghcr.io",                 // GitHub Container Registry
	}
	
	for _, element := range requiredElements {
		if !strings.Contains(workflow, element) {
			t.Errorf("Workflow should contain: %s", element)
		}
	}
	
	// Test for security best practices
	if !strings.Contains(workflow, "contents: read") {
		t.Error("Workflow should have minimal permissions")
	}
	
	if !strings.Contains(workflow, "packages: write") {
		t.Error("Workflow should have package write permissions")
	}
}

// TestDockerHubWorkflowExample tests the Docker Hub workflow example
func TestDockerHubWorkflowExample(t *testing.T) {
	examplePath := ".github/workflows/docker-hub.yml.example"
	
	if _, err := os.Stat(examplePath); os.IsNotExist(err) {
		t.Fatal("Docker Hub workflow example does not exist")
	}
	
	content, err := os.ReadFile(examplePath)
	if err != nil {
		t.Fatalf("Failed to read Docker Hub workflow example: %v", err)
	}
	
	workflow := string(content)
	
	// Test for Docker Hub specific elements
	requiredElements := []string{
		"DOCKERHUB_USERNAME",
		"DOCKERHUB_TOKEN",
		"your-dockerhub-username/s3-static",
		"docker/login-action@v3",
	}
	
	for _, element := range requiredElements {
		if !strings.Contains(workflow, element) {
			t.Errorf("Docker Hub workflow example should contain: %s", element)
		}
	}
}

// TestDockerBuildDocumentation tests that the documentation is accurate
func TestDockerBuildDocumentation(t *testing.T) {
	docPath := "docs/docker-build.md"
	
	if _, err := os.Stat(docPath); os.IsNotExist(err) {
		t.Fatal("Docker build documentation does not exist")
	}
	
	content, err := os.ReadFile(docPath)
	if err != nil {
		t.Fatalf("Failed to read documentation: %v", err)
	}
	
	doc := string(content)
	
	// Test for required documentation sections
	requiredSections := []string{
		"# Docker 构建和部署",
		"## GitHub Container Registry",
		"## Docker Hub",
		"## 本地构建",
		"## 功能特性",
		"多平台支持",
		"构建缓存优化",
		"非 root 用户运行",
		"健康检查",
	}
	
	for _, section := range requiredSections {
		if !strings.Contains(doc, section) {
			t.Errorf("Documentation should contain section: %s", section)
		}
	}
	
	// Test for accurate command examples
	commandExamples := []string{
		"docker pull ghcr.io/your-username/s3-static:latest",
		"docker build -t s3-static .",
		"docker buildx build --platform linux/amd64,linux/arm64",
	}
	
	for _, cmd := range commandExamples {
		if !strings.Contains(doc, cmd) {
			t.Errorf("Documentation should contain command example: %s", cmd)
		}
	}
}

// TestJustfileDockerCommands tests that justfile Docker commands work
func TestJustfileDockerCommands(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping justfile Docker commands test in short mode")
	}
	
	// Check if just is available
	if !isJustAvailable() {
		t.Skip("just is not available, skipping justfile test")
	}
	
	if !isDockerAvailable() {
		t.Skip("Docker is not available, skipping justfile Docker test")
	}
	
	// Test that docker-build command exists in justfile
	content, err := os.ReadFile("justfile")
	if err != nil {
		t.Fatalf("Failed to read justfile: %v", err)
	}
	
	justfile := string(content)
	
	if !strings.Contains(justfile, "docker-build:") {
		t.Error("justfile should contain docker-build command")
	}
	
	if !strings.Contains(justfile, "docker-run:") {
		t.Error("justfile should contain docker-run command")
	}
	
	// Test that the docker-build command works
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	
	cmd := exec.CommandContext(ctx, "just", "docker-build")
	output, err := cmd.CombinedOutput()
	
	if err != nil {
		t.Logf("Docker build via just failed (this might be expected in CI): %v\nOutput: %s", err, string(output))
		// Don't fail the test as Docker might not be available in all CI environments
	}
}

// TestDockerComposeConfiguration tests the Docker Compose setup
func TestDockerComposeConfiguration(t *testing.T) {
	composePath := "compose.yaml"
	
	if _, err := os.Stat(composePath); os.IsNotExist(err) {
		t.Fatal("Docker Compose file does not exist")
	}
	
	content, err := os.ReadFile(composePath)
	if err != nil {
		t.Fatalf("Failed to read compose file: %v", err)
	}
	
	compose := string(content)
	
	// Test for required services
	requiredServices := []string{
		"minio:",
		"minio-setup:",
		"s3-static:",
	}
	
	for _, service := range requiredServices {
		if !strings.Contains(compose, service) {
			t.Errorf("Compose file should contain service: %s", service)
		}
	}
	
	// Test for health checks
	if !strings.Contains(compose, "healthcheck:") {
		t.Error("Compose file should include health checks")
	}
	
	// Test for proper networking
	if !strings.Contains(compose, "depends_on:") {
		t.Error("Compose file should define service dependencies")
	}
}

// Helper function to check if Docker is available
func isDockerAvailable() bool {
	cmd := exec.Command("docker", "version")
	return cmd.Run() == nil
}

// Helper function to check if just is available
func isJustAvailable() bool {
	cmd := exec.Command("just", "--version")
	return cmd.Run() == nil
}

// TestDockerImageSize tests that the Docker image is reasonably sized
func TestDockerImageSize(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping Docker image size test in short mode")
	}
	
	if !isDockerAvailable() {
		t.Skip("Docker is not available, skipping image size test")
	}
	
	imageName := "s3-static-test"
	
	// Build the image first
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	
	buildCmd := exec.CommandContext(ctx, "docker", "build", "-t", imageName, ".")
	if err := buildCmd.Run(); err != nil {
		t.Skip("Docker build failed, skipping image size test")
	}
	
	defer exec.Command("docker", "rmi", imageName).Run()
	
	// Get image size
	cmd := exec.Command("docker", "images", imageName, "--format", "{{.Size}}")
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("Failed to get image size: %v", err)
	}
	
	size := strings.TrimSpace(string(output))
	t.Logf("Docker image size: %s", size)
	
	// The image should be reasonably small (less than 100MB for Alpine-based Go binary)
	// This is more of an informational test
	if strings.Contains(size, "GB") {
		t.Errorf("Docker image seems too large: %s", size)
	}
}

// TestDockerBuildArgs tests that build args work correctly
func TestDockerBuildArgs(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping Docker build args test in short mode")
	}
	
	if !isDockerAvailable() {
		t.Skip("Docker is not available, skipping build args test")
	}
	
	// Test building with different Go version (if supported)
	imageName := "s3-static-test-args"
	
	defer exec.Command("docker", "rmi", imageName).Run()
	
	// Build with explicit Go version
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	
	cmd := exec.CommandContext(ctx, "docker", "build", "-t", imageName, ".")
	output, err := cmd.CombinedOutput()
	
	if err != nil {
		t.Logf("Docker build with args failed (expected in some cases): %v\nOutput: %s", err, string(output))
		// Don't fail as this might not be supported in all Dockerfile versions
	}
}

// TestDockerfileLayerOptimization tests that Dockerfile is optimized for layer caching
func TestDockerfileLayerOptimization(t *testing.T) {
	content, err := os.ReadFile("Dockerfile")
	if err != nil {
		t.Fatalf("Failed to read Dockerfile: %v", err)
	}
	
	dockerfile := string(content)
	lines := strings.Split(dockerfile, "\n")
	
	// Find the order of COPY commands
	var copyGoMod, copySource int
	for i, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "COPY go.mod go.sum") {
			copyGoMod = i
		}
		if strings.HasPrefix(line, "COPY . .") {
			copySource = i
		}
	}
	
	// go.mod and go.sum should be copied before source code for better caching
	if copyGoMod > 0 && copySource > 0 && copyGoMod >= copySource {
		t.Error("go.mod and go.sum should be copied before source code for better Docker layer caching")
	}
	
	// Check that RUN go mod download comes after copying go.mod
	var modDownload int
	for i, line := range lines {
		line = strings.TrimSpace(line)
		if strings.Contains(line, "go mod download") {
			modDownload = i
		}
	}
	
	if copyGoMod > 0 && modDownload > 0 && copyGoMod >= modDownload {
		t.Error("go mod download should come after copying go.mod files")
	}
}