# 缓存策略配置

本服务支持三种缓存策略，可通过 `CACHE_STRATEGY` 环境变量配置。

## 缓存策略说明

### 1. immutable (推荐，默认)

```bash
export CACHE_STRATEGY=immutable
export CACHE_DURATION=8760h  # 1年
```

**适用场景**: 创建后不变的静态文件（如用户上传的图片、文档等）

**行为**:
- 浏览器在缓存期间内完全不发送任何请求
- 直接使用本地缓存，性能最佳
- 需要通过改变 URL 来更新内容

**网络请求示例**:
```
第一次: GET /image.jpg → 200 OK (下载内容)
第二次: GET /image.jpg → (无网络请求，直接使用缓存)
更新版本: GET /image.jpg?v=2 → 200 OK (下载新版本)
```

### 2. no-cache (适用于可变内容)

```bash
export CACHE_STRATEGY=max-age
export CACHE_DURATION=1h  # 设置缓存时间
```

**适用场景**: 仅用于测试或特殊需求

**行为**:
- 浏览器在 `max-age` 时间内会发送条件请求验证缓存
- 可能导致资源版本不匹配问题

**问题**: 如果你同时更新了多个相关文件（HTML、CSS、JS），用户可能获得新旧版本混合的资源，导致页面错乱。

### 3. immutable (适用于永不变化的内容)

```bash
export CACHE_STRATEGY=immutable
export CACHE_DURATION=8760h  # 1年
```

**适用场景**: 带版本号或哈希值的静态资源（如 `style.v1.2.3.css`、`script.abc123.js`）

**行为**:
- 浏览器在 `max-age` 期间内完全不发送任何请求
- 直接使用本地缓存，性能最佳
- 需要通过改变 URL 来更新内容

**网络请求示例**:
```
第一次: GET /style.v1.css → 200 OK (下载内容)
第二次: GET /style.v1.css → (无网络请求，直接使用缓存)
更新版本: GET /style.v2.css → 200 OK (下载新版本)
```

## 最佳实践建议

### 对于 S3 静态文件服务

由于你的服务主要用于托管用户上传的文件，且 99.9% 的文件创建后不会变化，建议使用：

```bash
export CACHE_STRATEGY=immutable
export CACHE_DURATION=8760h  # 1年
```

这样可以：
- ✅ 99.9% 的资源获得最佳性能（零网络请求）
- ✅ 大幅减少服务器负载和带宽消耗
- ✅ 用户体验更好（即时加载）
- ✅ 符合静态资源缓存最佳实践

### 处理 0.1% 可变资源的方案

1. **URL 版本化**：`/image.jpg?v=20250831`
2. **文件名版本化**：`image-v2.jpg`
3. **手动缓存清除**：通过 CDN 或代理清除特定文件缓存
4. **混合策略**：为特定路径使用不同缓存策略

### 如果你有版本控制的静态资源

如果你的文件名包含版本信息或哈希值（如通过构建工具生成），可以使用：

```bash
export CACHE_STRATEGY=immutable
export CACHE_DURATION=8760h  # 1年
```

## 配置示例

### Docker Compose 配置

```yaml
services:
  s3-static:
    image: s3-static:latest
    environment:
      - CACHE_STRATEGY=no-cache
      - S3_ENDPOINT=https://s3.amazonaws.com
      - BUCKET_NAME=my-bucket
    ports:
      - "8080:8080"
```

### 环境变量配置

```bash
# 推荐配置（可变内容）
export CACHE_STRATEGY=no-cache

# 或者用于不可变内容
export CACHE_STRATEGY=immutable
export CACHE_DURATION=8760h
```

## 验证缓存行为

你可以使用浏览器开发者工具的网络面板来验证缓存行为：

1. **no-cache**: 每次刷新都会看到请求，但如果内容未变化会返回 304
2. **immutable**: 刷新页面时不会看到网络请求（除非强制刷新 Ctrl+F5）

## 参考资料

- [MDN: HTTP Caching](https://developer.mozilla.org/en-US/docs/Web/HTTP/Caching)
- [Web.dev: HTTP Cache](https://web.dev/http-cache/)
- [Jake Archibald: Caching Best Practices](https://jakearchibald.com/2016/caching-best-practices/)