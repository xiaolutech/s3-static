package main

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

func main() {
	// 连接到 MinIO
	minioClient, err := minio.New("localhost:9000", &minio.Options{
		Creds:  credentials.NewStaticV4("minioadmin", "minioadmin", ""),
		Secure: false,
	})
	if err != nil {
		log.Fatalln(err)
	}

	// 1. 直接 HTTP 请求查看响应头
	fmt.Println("=== 直接 HTTP 请求 MinIO ===")
	resp, err := http.Get("http://localhost:9000/test-bucket/test-file.txt")
	if err != nil {
		log.Printf("HTTP 请求失败: %v", err)
	} else {
		fmt.Printf("Status: %s\n", resp.Status)
		fmt.Printf("x-amz-request-id: %s\n", resp.Header.Get("x-amz-request-id"))
		fmt.Printf("x-amz-id-2: %s\n", resp.Header.Get("x-amz-id-2"))
		fmt.Printf("Content-Type: %s\n", resp.Header.Get("Content-Type"))
		fmt.Printf("ETag: %s\n", resp.Header.Get("ETag"))
		fmt.Printf("Last-Modified: %s\n", resp.Header.Get("Last-Modified"))
		resp.Body.Close()
	}

	// 2. 使用 MinIO SDK 获取对象信息
	fmt.Println("\n=== 使用 MinIO SDK ===")
	ctx := context.Background()
	objInfo, err := minioClient.StatObject(ctx, "test-bucket", "test-file.txt", minio.StatObjectOptions{})
	if err != nil {
		log.Printf("StatObject 失败: %v", err)
	} else {
		fmt.Printf("ETag: %s\n", objInfo.ETag)
		fmt.Printf("Size: %d\n", objInfo.Size)
		fmt.Printf("LastModified: %s\n", objInfo.LastModified)
		fmt.Printf("ContentType: %s\n", objInfo.ContentType)
	}
}