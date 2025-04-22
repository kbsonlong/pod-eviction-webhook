package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
)

// NodePoolConfig 节点池配置
type NodePoolConfig struct {
	LabelSelector metav1.LabelSelector `json:"labelSelector"` // 节点标签选择器
	Threshold     int                  `json:"threshold"`     // NotReady节点数量阈值
	Window        time.Duration        `json:"window"`        // 检测时间窗口
}

// Config 应用配置
type Config struct {
	WebhookPort      int              `json:"webhookPort"`
	CertDir          string           `json:"certDir"`
	ConfigMapDir     string           `json:"configMapDir"`     // ConfigMap 挂载目录
	NodePools        []NodePoolConfig `json:"nodePools"`        // 节点池配置列表
	DefaultThreshold int              `json:"defaultThreshold"` // 默认阈值
	DefaultWindow    time.Duration    `json:"defaultWindow"`    // 默认时间窗口
}

// NewConfig 创建新的配置
func NewConfig() *Config {
	port, _ := strconv.Atoi(getEnv("WEBHOOK_PORT", "8443"))
	threshold, _ := strconv.Atoi(getEnv("NODE_NOTREADY_THRESHOLD", "3"))
	window, _ := strconv.Atoi(getEnv("NODE_NOTREADY_WINDOW", "300")) // 默认5分钟

	return &Config{
		WebhookPort:      port,
		CertDir:          getEnv("CERT_DIR", "/tmp/k8s-webhook-server/serving-certs"),
		ConfigMapDir:     getEnv("CONFIG_MAP_DIR", "/etc/webhook/config"),
		DefaultThreshold: threshold,
		DefaultWindow:    time.Duration(window) * time.Second,
		NodePools:        parseNodePoolsConfig(),
	}
}

// NewLocalConfig 创建本地开发配置
func NewLocalConfig() *Config {
	return &Config{
		WebhookPort:      8080,
		CertDir:          "",
		ConfigMapDir:     "./config",
		DefaultThreshold: 3,
		DefaultWindow:    5 * time.Minute,
		NodePools:        parseNodePoolsConfig(),
	}
}

// getEnv 获取环境变量，如果不存在则返回默认值
func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

// parseNodePoolsConfig 解析节点池配置
func parseNodePoolsConfig() []NodePoolConfig {
	// 从 ConfigMap 文件读取配置
	configFile := filepath.Join(getEnv("CONFIG_MAP_DIR", "/etc/webhook/config"), "node-pools.json")
	data, err := os.ReadFile(configFile)
	if err != nil {
		klog.Errorf("Failed to read node pools config file: %v", err)
		return []NodePoolConfig{}
	}

	var nodePools []NodePoolConfig
	if err := json.Unmarshal(data, &nodePools); err != nil {
		klog.Errorf("Failed to parse node pools config: %v", err)
		return []NodePoolConfig{}
	}

	return nodePools
}
