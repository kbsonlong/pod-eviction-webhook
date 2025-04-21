package webhook

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	admissionv1 "k8s.io/api/admission/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"

	"github.com/kbsonlong/webhook/pkg/monitor"
)

var (
	evictionInterceptedTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "eviction_intercepted_total",
		Help: "Total number of eviction requests intercepted",
	})
	evictionAllowedTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "eviction_allowed_total",
		Help: "Total number of eviction requests allowed",
	})
)

// Webhook handles admission requests
type Webhook struct {
	nodeMonitor *monitor.NodeMonitor
}

// NewWebhook creates a new Webhook instance
func NewWebhook(nodeMonitor *monitor.NodeMonitor) *Webhook {
	return &Webhook{
		nodeMonitor: nodeMonitor,
	}
}

// HandleAdmission handles admission requests
func (w *Webhook) HandleAdmission(c *gin.Context) {
	var admissionReview admissionv1.AdmissionReview
	if err := c.ShouldBindJSON(&admissionReview); err != nil {
		klog.Errorf("Failed to decode admission review: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Only handle DELETE operations for pods
	if admissionReview.Request.Operation != admissionv1.Delete {
		c.JSON(http.StatusOK, admissionReview)
		return
	}

	// Decode the pod being deleted
	var pod v1.Pod
	if err := json.Unmarshal(admissionReview.Request.OldObject.Raw, &pod); err != nil {
		klog.Errorf("Failed to decode pod: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Check if we should intercept the eviction
	shouldIntercept := w.nodeMonitor.ShouldInterceptEviction(&pod)

	// Update metrics
	if shouldIntercept {
		evictionInterceptedTotal.Inc()
	} else {
		evictionAllowedTotal.Inc()
	}

	// Prepare the response
	admissionResponse := &admissionv1.AdmissionResponse{
		UID:     admissionReview.Request.UID,
		Allowed: !shouldIntercept,
	}

	if shouldIntercept {
		admissionResponse.Result = &metav1.Status{
			Status:  "Failure",
			Message: "Pod eviction intercepted due to multiple nodes being NotReady",
			Reason:  "EvictionProtection",
			Code:    http.StatusForbidden,
		}
	}

	admissionReview.Response = admissionResponse
	c.JSON(http.StatusOK, admissionReview)
}
