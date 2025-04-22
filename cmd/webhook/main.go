package main

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/kbsonlong/webhook/pkg/config"
	"github.com/kbsonlong/webhook/pkg/handler"
	"github.com/kbsonlong/webhook/pkg/monitor"
	"github.com/kbsonlong/webhook/pkg/webhook"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
)

var (
	localMode = flag.Bool("local", false, "Run in local development mode")
)

func main() {
	// Parse command line flags
	flag.Parse()

	// Create configuration
	var cfg *config.Config
	if *localMode {
		cfg = config.NewLocalConfig()
	} else {
		cfg = config.NewConfig()
	}

	// Create Kubernetes client
	var kubeConfig *rest.Config
	var err error

	if *localMode {
		// Use local kubeconfig
		kubeConfig, err = clientcmd.BuildConfigFromFlags("", filepath.Join(os.Getenv("HOME"), ".kube", "config"))
		if err != nil {
			klog.Fatalf("Failed to create local kubeconfig: %v", err)
		}
	} else {
		// Use in-cluster config
		kubeConfig, err = rest.InClusterConfig()
		if err != nil {
			klog.Fatalf("Failed to create in-cluster config: %v", err)
		}
	}

	clientset, err := kubernetes.NewForConfig(kubeConfig)
	if err != nil {
		klog.Fatalf("Failed to create Kubernetes client: %v", err)
	}

	// Create callback handler
	callbackHandler := handler.NewCallbackHandler()

	// Create node monitor
	nodeMonitor := monitor.NewNodeMonitor(clientset, cfg, callbackHandler)

	// Create webhook handler
	webhookHandler := webhook.NewWebhook(nodeMonitor, clientset)

	// Create Gin router
	router := gin.Default()

	// Add health check endpoint
	router.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// Add webhook endpoint
	router.POST("/validate", webhookHandler.HandleAdmission)

	// Add callback endpoint
	callbackHandler.RegisterRoutes(router)

	// Create HTTP server
	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.WebhookPort),
		Handler: router,
		TLSConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
		},
	}

	// Start node monitor
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := nodeMonitor.Start(ctx); err != nil {
		klog.Fatalf("Failed to start node monitor: %v", err)
	}

	// Start server in a goroutine
	go func() {
		klog.Infof("Starting webhook server on port %d", cfg.WebhookPort)
		if *localMode {
			// In local mode, we don't use TLS for simplicity
			if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				klog.Fatalf("Failed to start server: %v", err)
			}
		} else {
			// In-cluster mode uses TLS
			if err := server.ListenAndServeTLS(
				cfg.CertDir+"/tls.crt",
				cfg.CertDir+"/tls.key",
			); err != nil && err != http.ErrServerClosed {
				klog.Fatalf("Failed to start server: %v", err)
			}
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	// Graceful shutdown
	klog.Info("Shutting down server...")
	ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		klog.Fatalf("Server forced to shutdown: %v", err)
	}

	klog.Info("Server exiting")
}
