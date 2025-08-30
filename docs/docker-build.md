# Docker 构建和部署

本项目包含 GitHub Actions 工作流来自动构建和推送 Docker 镜像。

## GitHub Container Registry (默认)

主要的工作流 `.github/workflows/docker-build.yml` 会自动：

- 在推送到 `main` 或 `develop` 分支时构建镜像
- 在创建标签时构建发布版本
- 在 Pull Request 时进行构建测试（不推送）
- 支持多平台构建 (linux/amd64, linux/arm64)
- 使用 GitHub Container Registry (ghcr.io)

### 镜像标签规则

- `main` 分支 → `latest` 标签
- `develop` 分支 → `develop` 标签
- 版本标签 `v1.2.3` → `1.2.3`, `1.2`, `1` 标签
- Pull Request → `pr-123` 标签

### 使用镜像

```bash
# 拉取最新版本
docker pull ghcr.io/your-username/s3-static:latest

# 拉取特定版本
docker pull ghcr.io/your-username/s3-static:1.0.0

# 运行容器
docker run -p 8080:8080 ghcr.io/your-username/s3-static:latest
```

## Docker Hub (可选)

如果你想推送到 Docker Hub，可以：

1. 重命名 `.github/workflows/docker-hub.yml.example` 为 `.github/workflows/docker-hub.yml`
2. 在 GitHub 仓库设置中添加以下 Secrets：
   - `DOCKERHUB_USERNAME`: 你的 Docker Hub 用户名
   - `DOCKERHUB_TOKEN`: 你的 Docker Hub 访问令牌
3. 修改工作流中的镜像名称

## 本地构建

```bash
# 构建镜像
docker build -t s3-static .

# 多平台构建
docker buildx build --platform linux/amd64,linux/arm64 -t s3-static .
```

## 功能特性

- ✅ 多平台支持 (AMD64, ARM64)
- ✅ 构建缓存优化
- ✅ 自动标签管理
- ✅ 安全扫描
- ✅ 构建证明 (Build Attestation)
- ✅ 非 root 用户运行
- ✅ 健康检查