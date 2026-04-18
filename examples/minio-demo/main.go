package main

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
)

func main() {
	cfg, err := awsconfig.LoadDefaultConfig(context.Background(),
		awsconfig.WithRegion("us-east-1"),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider("minioadmin", "minioadmin", "")),
	)
	if err != nil {
		log.Fatalln(err)
	}

	s3Client := awss3.NewFromConfig(cfg, func(o *awss3.Options) {
		o.UsePathStyle = true
		o.BaseEndpoint = aws.String("http://localhost:9000")
	})

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

	fmt.Println("\n=== 使用 AWS SDK v2 ===")
	objInfo, err := s3Client.HeadObject(context.Background(), &awss3.HeadObjectInput{
		Bucket: aws.String("test-bucket"),
		Key:    aws.String("test-file.txt"),
	})
	if err != nil {
		log.Printf("HeadObject 失败: %v", err)
	} else {
		fmt.Printf("ETag: %s\n", aws.ToString(objInfo.ETag))
		fmt.Printf("Size: %d\n", aws.ToInt64(objInfo.ContentLength))
		if objInfo.LastModified != nil {
			fmt.Printf("LastModified: %s\n", objInfo.LastModified.String())
		}
		fmt.Printf("ContentType: %s\n", aws.ToString(objInfo.ContentType))
	}
}
