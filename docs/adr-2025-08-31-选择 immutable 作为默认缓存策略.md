# ADR: 选择 immutable 作为默认缓存策略

**状态**: 已接受  
**日期**: 2025-08-31  
**决策者**: 开发团队  

## 背景

在实现可配置缓存策略系统后，需要为 S3 静态文件服务选择合适的默认缓存策略。

### 业务场景分析

- **99.9% 的资源**：用户上传后不会再变化（图片、文档、媒体文件等）
- **0.1% 的资源**：可能需要更新或替换
- **服务性质**：主要用于静态文件托管，类似 CDN

### 可选策略

1. **no-cache**: 每次验证缓存，确保内容新鲜
2. **max-age**: 在指定时间内可能提供过期内容
3. **immutable**: 在缓存期内完全不发送请求

## 决策

**选择 `immutable` 作为默认缓存策略，缓存时间设置为 1 年（8760 小时）。**

### 配置

```bash
export CACHE_STRATEGY=immutable
export CACHE_DURATION=8760h  # 1年
```

## 理由

### 性能优势

1. **零网络请求**: 99.9% 的资源在缓存期内不会产生任何网络请求
2. **服务器负载降低**: 大幅减少服务器处理的请求数量
3. **带宽节省**: 显著降低带宽消耗
4. **用户体验**: 资源即时加载，无等待时间

### 成本效益

- **服务器成本**: 减少 CPU 和内存使用
- **带宽成本**: 大幅降低出站流量费用
- **CDN 成本**: 如果使用 CDN，可显著降低回源请求

### 数据支持

基于 **99.9% 资源不变** 的业务特征：
- 使用 `no-cache`: 100% 的请求需要验证 → 100% 网络请求
- 使用 `immutable`: 99.9% 的请求无需网络 → 0.1% 网络请求
- **性能提升**: 约 1000 倍的网络请求减少

## 处理 0.1% 可变资源的策略

### 推荐方案

1. **URL 版本化**
   ```
   原始: /document.pdf
   更新: /document.pdf?v=20250831
   ```

2. **文件名版本化**
   ```
   原始: document.pdf
   更新: document-v2.pdf
   ```

3. **时间戳方案**
   ```
   原始: /image.jpg
   更新: /image.jpg?t=1693478400
   ```

### 高级方案

4. **CDN 缓存清除**: 通过 CDN API 清除特定文件缓存
5. **混合策略**: 为特定路径模式使用不同缓存策略（未来扩展）

## 风险与缓解

### 风险

1. **内容更新困难**: 更新文件后用户可能看到旧版本
2. **调试复杂性**: 缓存问题可能难以排查

### 缓解措施

1. **文档说明**: 在文档中明确说明更新策略
2. **工具支持**: 提供缓存清除工具或脚本
3. **监控告警**: 监控缓存命中率和更新频率
4. **回退机制**: 可随时切换到 `no-cache` 策略

## 实现细节

### 代码变更

```go
// internal/config/config.go
func DefaultConfig() *Config {
    return &Config{
        // ...
        DefaultCacheDuration: time.Hour * 24 * 365, // 1年
        CacheStrategy:        "immutable",          // 默认策略
    }
}
```

### 响应头示例

```http
HTTP/1.1 200 OK
Cache-Control: max-age=31536000, immutable
ETag: "d48fbb3c6cfde4616e2cd60d1f4ef728"
Last-Modified: Fri, 13 Jun 2025 16:28:20 GMT
```

## 监控指标

### 关键指标

1. **缓存命中率**: 目标 > 99%
2. **带宽使用**: 预期减少 90%+
3. **服务器负载**: 预期减少 90%+
4. **用户投诉**: 关于内容更新的问题

### 告警阈值

- 缓存命中率 < 95%
- 带宽使用异常增长
- 内容更新相关的用户反馈

## 替代方案

### 如果业务场景变化

1. **可变内容增加**: 切换到 `no-cache`
2. **混合场景**: 实现路径级别的缓存策略
3. **特殊需求**: 使用 `max-age` 策略

### 配置切换

```bash
# 切换到保守策略
export CACHE_STRATEGY=no-cache

# 切换到中等策略
export CACHE_STRATEGY=max-age
export CACHE_DURATION=1h
```

## 结论

基于 **99.9% 资源不变** 的业务特征，选择 `immutable` 策略可以：

- 🚀 **性能提升**: 约 1000 倍的网络请求减少
- 💰 **成本节省**: 显著降低服务器和带宽成本
- 😊 **用户体验**: 资源即时加载
- 🔧 **可维护性**: 通过 URL 版本化处理更新需求

这个决策符合静态文件服务的最佳实践，并为业务带来显著的性能和成本优势。

## 参考资料

- [MDN: HTTP Caching](https://developer.mozilla.org/en-US/docs/Web/HTTP/Caching)
- [Web.dev: HTTP Cache](https://web.dev/http-cache/)
- [Jake Archibald: Caching Best Practices](https://jakearchibald.com/2016/caching-best-practices/)
- [RFC 9111: HTTP Caching](https://httpwg.org/specs/rfc9111.html)