# justfile 使用指南

本项目使用 [just](https://github.com/casey/just) 作为任务运行器，替代传统的 Makefile。

## 安装 just

### macOS
```bash
brew install just
```

### Linux
```bash
# 使用 cargo
cargo install just

# 或者下载预编译二进制文件
curl --proto '=https' --tlsv1.2 -sSf https://just.systems/install.sh | bash -s -- --to ~/bin
```

### Windows
```bash
# 使用 scoop
scoop install just

# 或者使用 cargo
cargo install just
```

## 常用命令

### 查看所有可用命令
```bash
just
# 或者
just --list
```

### 测试相关
```bash
# 运行所有测试
just test

# 运行单元测试
just test-unit

# 运行集成测试
just test-integration

# 运行基准测试
just test-benchmark

# 生成覆盖率报告
just test-coverage

# 在浏览器中查看覆盖率报告
just coverage-html

# 运行详细的覆盖率分析脚本
just coverage-script

# 运行覆盖率分析脚本（包含基准测试）
just coverage-script-with-benchmarks
```

### 构建相关
```bash
# 构建应用程序
just build

# 构建所有平台
just build-all

# 运行应用程序
just run

# 开发模式运行
just run-dev
```

### 代码质量
```bash
# 格式化代码
just fmt

# 运行 linter
just lint

# 运行 go vet
just vet

# 快速开发周期（格式化 + vet + 单元测试）
just dev

# 完整验证（用于 CI）
just validate

# 预提交检查
just pre-commit
```

### 性能分析
```bash
# 运行性能测试
just perf

# CPU 性能分析
just profile-cpu

# 内存性能分析
just profile-mem
```

### 开发环境
```bash
# 设置开发环境
just setup

# 安装开发工具
just install-tools

# 整理依赖
just mod-tidy

# 检查依赖更新
just deps-check
```

### Docker
```bash
# 构建 Docker 镜像
just docker-build

# 运行 Docker 容器
just docker-run
```

### 清理
```bash
# 清理构建产物
just clean

# 深度清理
just clean-all
```

### 其他
```bash
# 显示项目信息
just info

# 安全扫描
just security

# 生成代码
just generate
```

## justfile 的优势

相比 Makefile，justfile 有以下优势：

1. **更简洁的语法**: 不需要处理 tab 和空格的问题
2. **更好的错误信息**: 提供更清晰的错误提示
3. **跨平台兼容**: 在 Windows、macOS 和 Linux 上都能很好工作
4. **现代化**: 支持更多现代化的功能和语法
5. **易于维护**: 语法更直观，更容易理解和维护

## 示例工作流

### 日常开发
```bash
# 开始开发前
just setup

# 开发过程中
just dev

# 提交前检查
just pre-commit
```

### CI/CD
```bash
# CI 测试
just ci-test

# CI linting
just ci-lint

# 发布准备
just release-prep
```

### 性能监控
```bash
# 运行基准测试
just test-benchmark

# 详细性能分析
just perf
```