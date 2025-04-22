package webhook

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	admissionv1 "k8s.io/api/admission/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
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
	clientset   *kubernetes.Clientset
}

// NewWebhook creates a new Webhook instance
func NewWebhook(nodeMonitor *monitor.NodeMonitor, clientset *kubernetes.Clientset) *Webhook {
	return &Webhook{
		nodeMonitor: nodeMonitor,
		clientset:   clientset,
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

	// Only handle DELETE and UPDATE operations for pods
	if admissionReview.Request.Operation != admissionv1.Delete &&
		admissionReview.Request.Operation != admissionv1.Update {
		c.JSON(http.StatusOK, admissionReview)
		return
	}

	// Decode the pod being deleted or updated
	var pod v1.Pod
	if admissionReview.Request.Operation == admissionv1.Delete {
		if err := json.Unmarshal(admissionReview.Request.OldObject.Raw, &pod); err != nil {
			klog.Errorf("Failed to decode pod: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
	} else {
		if err := json.Unmarshal(admissionReview.Request.Object.Raw, &pod); err != nil {
			klog.Errorf("Failed to decode pod: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
	}

	// Check if we should intercept the eviction
	shouldIntercept := w.nodeMonitor.ShouldInterceptEviction(&pod)

	// Update metrics
	if shouldIntercept {
		evictionInterceptedTotal.Inc()
		// Create event for the pod
		event := &v1.Event{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "pod-eviction-protection-",
			},
			InvolvedObject: v1.ObjectReference{
				Kind:      "Pod",
				Name:      pod.Name,
				Namespace: pod.Namespace,
				UID:       pod.UID,
			},
			Reason:  "EvictionProtection",
			Message: "Pod eviction intercepted due to node being NotReady. Waiting for administrator confirmation.",
			Source: v1.EventSource{
				Component: "pod-eviction-protection",
			},
			FirstTimestamp: metav1.NewTime(time.Now()),
			LastTimestamp:  metav1.NewTime(time.Now()),
			Count:          1,
			Type:           "Warning",
		}

		_, err := w.clientset.CoreV1().Events(pod.Namespace).Create(c.Request.Context(), event, metav1.CreateOptions{})
		if err != nil {
			klog.Errorf("Failed to create event for pod %s/%s: %v", pod.Namespace, pod.Name, err)
		} else {
			klog.Infof("Created event for pod %s/%s", pod.Namespace, pod.Name)
		}
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
