# S3-Static 视频流式加载与秒开优化说明

## 背景

在 `s3-static` 原有的架构设计中，文件下载逻辑采用的是全量读取模式。也就是说，当用户发起文件请求时，服务端会通过 `io.ReadAll` 将底层的 MinIO S3 对象数据一次性全部读取到服务器内存中，然后再写入 HTTP 响应体。

这种方式由于以下两点原因对视频加载（特别是高清、大体积视频）极为不友好：

1. **极其消耗内存**：请求一个 1GB 的视频将会导致服务器内存瞬时吃紧，并发稍微高一点直接 OOM 崩溃。
2. **缺乏 Range 支持导致无法有效拖拽进度条**：流媒体播放器（例如 Chrome/Safari 的 `<video>` 标签）需要支持 HTTP `Range` 进行分段预加载。在之前的代码实现中强制返回 HTTP 200 并写回全尺寸视频，使得用户无法拖拽，甚至由于未返回 HTTP 206 Partial Content，部分严格的浏览器（如 Safari）会直接拒绝播放视频。

## 优化方案

为了彻底解决此问题，支持大型视频（或者任何大文件）的极速加载、降低内存占用，并完美实现诸如视频断点播放和进度条拖动等功能，本次加入了流式读取及分段支持方案，具体改造如下：

### 1. 废除 `ReadAll` 引入流式 `ReadSeekCloser`

去除了原先 `pkg/interfaces/storage.go` 中针对小文件场景设计的 `ReadFile([]byte, error)` 的依赖，新增并使用了 `GetFileReader(path string) (io.ReadSeekCloser, error)`。

S3 对象存储底层实现 `minio.Object` 原本就完美满足了标准库的 `io.ReadSeekCloser` 接口。我们将该指针直接传回 HTTP Handler 层，使其只在实际发起 HTTP 写入操作时才去产生 I/O 请求，而非一次性囤积数据到内存。

### 2. 利用 Golang 标准库 `http.ServeContent` 

移除了手动写入并替换为 `http.ServeContent`，原因在于这套原生的库极其强大：
- 自动解析并消化 `Range: bytes=X-Y` 头信息。
- 利用基于我们新引入流的 `Seek` 能力精准地跳往未下载大文件的指定字节位置。
- 自动组装并返回合规标准的 `206 Partial Content` 状态以及 `Content-Range` HTTP 表头。

### 何为 “秒开”？

采用流式解析由于只提供需要播放当下的视频片段数据即可开始渲染帧，使得前端 `<video>` 获取的首包立刻被解码和播映，完全省去了服务器下载完整 GB 级别对象的无效等待。从而让 s3-static 处理媒体文件有与商业 CDN 相当的响应能力和断点支持能力。
