package main

import (
	"fmt"
	"log"

	"s3-static/internal/storage"
)

func main() {
	// Example S3 configuration
	cfg := storage.S3Config{
		Endpoint:        "http://localhost:9000", // MinIO endpoint
		Region:          "us-east-1",
		AccessKeyID:     "minioadmin",
		SecretAccessKey: "minioadmin",
		Bucket:          "my-bucket",
		UseSSL:          false,
	}

	// Create S3 storage instance
	s3Storage, err := storage.NewS3Storage(cfg)
	if err != nil {
		log.Fatalf("Failed to create S3 storage: %v", err)
	}

	// Example usage
	testFile := "test.txt"

	// Check if file exists
	if s3Storage.FileExists(testFile) {
		fmt.Printf("File %s exists\n", testFile)

		// Get file info
		info, err := s3Storage.GetFileInfo(testFile)
		if err != nil {
			log.Printf("Failed to get file info: %v", err)
		} else {
			fmt.Printf("File size: %d bytes\n", info.Size)
			fmt.Printf("Last modified: %v\n", info.ModTime)
		}

		// Read file content
		content, err := s3Storage.ReadFile(testFile)
		if err != nil {
			log.Printf("Failed to read file: %v", err)
		} else {
			fmt.Printf("File content: %s\n", string(content))
		}
	} else {
		fmt.Printf("File %s does not exist\n", testFile)
	}
}