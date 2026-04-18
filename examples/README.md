# Examples

这个目录包含了 S3 Static File Service 的使用示例。

## 可用示例

### 1. S3 使用示例 (`s3-usage/`)
演示如何使用项目的 S3 存储接口：
- 连接到 S3 兼容存储（如 MinIO）
- 检查文件是否存在
- 获取文件信息
- 读取文件内容

运行方式：
```bash
# 使用 just
just run-s3-example

# 或直接使用 go
go run ./examples/s3-usage/
```

### 2. MinIO 头部演示 (`minio-demo/`)
演示如何直接与 MinIO 交互并检查 HTTP 响应头：
- 直接 HTTP 请求查看响应头
- 使用 AWS SDK v2 获取对象信息
- 比较不同方法获取的元数据

运行方式：
```bash
# 使用 just
just run-minio-example

# 或直接使用 go
go run ./examples/minio-demo/
```

## 前置条件

运行这些示例之前，请确保：

1. **MinIO 服务器运行中**：
   ```bash
   # 使用 Docker 启动 MinIO
   docker run -p 9000:9000 -p 9001:9001 \
     -e "MINIO_ROOT_USER=minioadmin" \
     -e "MINIO_ROOT_PASSWORD=minioadmin" \
     minio/minio server /data --console-address ":9001"
   ```

2. **创建测试桶和文件**：
   - 访问 MinIO 控制台：http://localhost:9001
   - 使用 minioadmin/minioadmin 登录
   - 创建名为 `test-bucket` 的桶
   - 上传一个名为 `test-file.txt` 的测试文件

## 配置

示例使用以下默认配置：
- **Endpoint**: http://localhost:9000
- **Access Key**: minioadmin
- **Secret Key**: minioadmin
- **Bucket**: my-bucket (s3-usage) / test-bucket (minio-demo)
- **SSL**: 禁用

如需修改配置，请编辑相应示例目录下的 `main.go` 文件。