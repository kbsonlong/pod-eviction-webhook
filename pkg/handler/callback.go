package handler

import (
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
	"k8s.io/klog/v2"
)

// CallbackHandler 处理解除拦截的回调请求
type CallbackHandler struct {
	mu            sync.RWMutex
	intercepting  bool
	notReadyNodes map[string]struct{}
}

// NewCallbackHandler 创建一个新的 CallbackHandler
func NewCallbackHandler() *CallbackHandler {
	return &CallbackHandler{
		notReadyNodes: make(map[string]struct{}),
	}
}

// RegisterRoutes 注册回调路由
func (h *CallbackHandler) RegisterRoutes(router *gin.Engine) {
	router.POST("/callback/disable-interception", h.DisableInterception)
	router.POST("/callback/enable-interception", h.EnableInterception)
	router.GET("/callback/status", h.GetStatus)
}

// DisableInterception 禁用拦截
func (h *CallbackHandler) DisableInterception(c *gin.Context) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.intercepting = false
	h.notReadyNodes = make(map[string]struct{})
	klog.Infof("Interception disabled via callback")
	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Interception disabled successfully",
	})
}

// EnableInterception 启用拦截
func (h *CallbackHandler) EnableInterception(c *gin.Context) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.intercepting = true
	klog.Infof("Interception enabled via callback")
	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Interception enabled successfully",
	})
}

// GetStatus 获取当前拦截状态
func (h *CallbackHandler) GetStatus(c *gin.Context) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data": gin.H{
			"intercepting":  h.intercepting,
			"notReadyNodes": h.getNotReadyNodeNames(),
		},
	})
}

// IsIntercepting 检查是否正在拦截
func (h *CallbackHandler) IsIntercepting() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.intercepting
}

// AddNotReadyNode 添加 NotReady 节点
func (h *CallbackHandler) AddNotReadyNode(nodeName string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.notReadyNodes[nodeName] = struct{}{}
}

// RemoveNotReadyNode 移除 NotReady 节点
func (h *CallbackHandler) RemoveNotReadyNode(nodeName string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.notReadyNodes, nodeName)
}

// getNotReadyNodeNames 获取所有 NotReady 节点名称
func (h *CallbackHandler) getNotReadyNodeNames() []string {
	names := make([]string, 0, len(h.notReadyNodes))
	for name := range h.notReadyNodes {
		names = append(names, name)
	}
	return names
}
