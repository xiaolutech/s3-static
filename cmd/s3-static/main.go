package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"s3-static/internal/config"
	"s3-static/internal/handler"
	"s3-static/internal/storage"
	"s3-static/pkg/interfaces"
)

func main() {
	fmt.Println("S3 Static File Service")

	// Initialize configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Initialize logger
	logger := config.NewLogger(cfg.LogLevel)

	// Initialize storage layer
	storageInstance, err := storage.NewStorage(cfg)
	if err != nil {
		logger.Fatal("Failed to initialize storage", map[string]interface{}{
			"error": err.Error(),
		})
	}

	var optimizedStorage interfaces.Storage
	if cfg.OptimizedImageEnabled {
		optimizedStorage, err = storage.NewS3StorageForBucket(cfg, cfg.OptimizedBucketName)
		if err != nil {
			logger.Error("Optimized image storage disabled", map[string]interface{}{
				"bucket": cfg.OptimizedBucketName,
				"error":  err.Error(),
			})
			optimizedStorage = nil
		}
	}

	// Initialize HTTP handlers
	fileHandler := handler.NewFileHandlerWithOptimizedStorage(storageInstance, optimizedStorage, cfg, logger)
	healthHandler := handler.NewHealthHandler(storageInstance, logger)

	// Setup HTTP server
	mux := http.NewServeMux()
	mux.Handle("/health", healthHandler)
	mux.Handle("/", fileHandler)

	server := &http.Server{
		Addr:    fmt.Sprintf("%s:%s", cfg.Host, cfg.Port),
		Handler: mux,
	}

	// Start server with graceful shutdown
	go func() {
		logger.Info("Server starting", map[string]interface{}{
			"address": server.Addr,
			"bucket":  cfg.BucketName,
		})

		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("Server failed to start", map[string]interface{}{
				"error": err.Error(),
			})
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Server shutting down", nil)

	// Give outstanding requests 30 seconds to complete
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		logger.Fatal("Server forced to shutdown", map[string]interface{}{
			"error": err.Error(),
		})
	}

	logger.Info("Server exited", nil)
}
