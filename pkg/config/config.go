package config

import (
	"os"
	"strconv"
	"time"
)

// Config holds the webhook configuration
type Config struct {
	WebhookPort           int
	CertDir               string
	NodeNotReadyThreshold int
	NodeNotReadyWindow    time.Duration
}

// NewConfig creates a new Config instance with default values
func NewConfig() *Config {
	return &Config{
		WebhookPort:           getEnvInt("WEBHOOK_PORT", 8443),
		CertDir:               getEnvString("CERT_DIR", "/tmp/k8s-webhook-server/serving-certs"),
		NodeNotReadyThreshold: getEnvInt("NODE_NOTREADY_THRESHOLD", 3),
		NodeNotReadyWindow:    getEnvDuration("NODE_NOTREADY_WINDOW", 5*time.Minute),
	}
}

// NewLocalConfig creates a new Config instance for local development
func NewLocalConfig() *Config {
	return &Config{
		WebhookPort:           getEnvInt("WEBHOOK_PORT", 8443),
		CertDir:               getEnvString("CERT_DIR", "./certs"),
		NodeNotReadyThreshold: getEnvInt("NODE_NOTREADY_THRESHOLD", 3),
		NodeNotReadyWindow:    getEnvDuration("NODE_NOTREADY_WINDOW", 5*time.Minute),
	}
}

// getEnvString returns the value of the environment variable or the default value
func getEnvString(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvInt returns the value of the environment variable as an integer or the default value
func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

// getEnvDuration returns the value of the environment variable as a duration or the default value
func getEnvDuration(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	return defaultValue
}
