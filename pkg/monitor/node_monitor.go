package monitor

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
)

var (
	nodeNotReadyCount = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "node_notready_count",
		Help: "Number of nodes in NotReady state",
	})
)

// NodeMonitor monitors the state of nodes in the cluster
type NodeMonitor struct {
	clientset     *kubernetes.Clientset
	notReadyNodes map[string]time.Time
	mu            sync.RWMutex
	threshold     int
	window        time.Duration
}

// NewNodeMonitor creates a new NodeMonitor instance
func NewNodeMonitor(clientset *kubernetes.Clientset, threshold int, window time.Duration) *NodeMonitor {
	return &NodeMonitor{
		clientset:     clientset,
		notReadyNodes: make(map[string]time.Time),
		threshold:     threshold,
		window:        window,
	}
}

// Start begins monitoring nodes
func (m *NodeMonitor) Start(ctx context.Context) error {
	// Create a node informer
	nodeInformer := cache.NewSharedIndexInformer(
		cache.NewListWatchFromClient(m.clientset.CoreV1().RESTClient(), "nodes", "", fields.Everything()),
		&v1.Node{},
		0,
		cache.Indexers{},
	)

	// Add event handlers
	nodeInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    m.handleNodeAdd,
		UpdateFunc: m.handleNodeUpdate,
		DeleteFunc: m.handleNodeDelete,
	})

	// Start the informer
	go nodeInformer.Run(ctx.Done())

	// Wait for the cache to sync
	if !cache.WaitForCacheSync(ctx.Done(), nodeInformer.HasSynced) {
		return fmt.Errorf("failed to sync node cache")
	}

	return nil
}

// ShouldInterceptEviction checks if eviction should be intercepted
func (m *NodeMonitor) ShouldInterceptEviction(pod *v1.Pod) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// If the pod is not on a NotReady node, allow eviction
	if pod.Spec.NodeName == "" {
		klog.Infof("Pod %s/%s has no node assigned, allowing eviction", pod.Namespace, pod.Name)
		return false
	}
	klog.Infof("Checking pod %s/%s on node: %s", pod.Namespace, pod.Name, pod.Spec.NodeName)

	// Check if the node is in our NotReady list
	timestamp, exists := m.notReadyNodes[pod.Spec.NodeName]
	if !exists {
		klog.Infof("Node %s is Ready, allowing eviction for pod %s/%s",
			pod.Spec.NodeName, pod.Namespace, pod.Name)
		return false
	}
	klog.Infof("Node %s is in NotReady list since %v", pod.Spec.NodeName, timestamp)

	// Count nodes that have been NotReady for less than the window duration
	count := 0
	now := time.Now()
	for nodeName, ts := range m.notReadyNodes {
		timeSinceNotReady := now.Sub(ts)
		if timeSinceNotReady < m.window {
			count++
			klog.Infof("Node %s has been NotReady for %v (within window of %v)",
				nodeName, timeSinceNotReady, m.window)
		} else {
			klog.Infof("Node %s has been NotReady for %v (outside window of %v)",
				nodeName, timeSinceNotReady, m.window)
		}
	}
	klog.Infof("Total NotReady nodes within window: %d, threshold: %d", count, m.threshold)

	// Update metrics
	nodeNotReadyCount.Set(float64(count))

	shouldIntercept := count >= m.threshold
	klog.Infof("Should intercept eviction for pod %s/%s: %v",
		pod.Namespace, pod.Name, shouldIntercept)

	return shouldIntercept
}

// handleNodeAdd handles node addition events
func (m *NodeMonitor) handleNodeAdd(obj interface{}) {
	node := obj.(*v1.Node)
	m.updateNodeStatus(node)
}

// handleNodeUpdate handles node update events
func (m *NodeMonitor) handleNodeUpdate(oldObj, newObj interface{}) {
	node := newObj.(*v1.Node)
	m.updateNodeStatus(node)
}

// handleNodeDelete handles node deletion events
func (m *NodeMonitor) handleNodeDelete(obj interface{}) {
	node := obj.(*v1.Node)
	m.mu.Lock()
	delete(m.notReadyNodes, node.Name)
	m.mu.Unlock()
}

// updateNodeStatus updates the node status in our tracking
func (m *NodeMonitor) updateNodeStatus(node *v1.Node) {
	m.mu.Lock()
	defer m.mu.Unlock()

	isNotReady := false
	var notReadyCondition *v1.NodeCondition
	for i, condition := range node.Status.Conditions {
		if condition.Type == v1.NodeReady {
			if condition.Status != v1.ConditionTrue {
				isNotReady = true
				notReadyCondition = &node.Status.Conditions[i]
			}
			break
		}
	}

	if isNotReady && notReadyCondition != nil {
		klog.Infof("Node %s is NotReady: Status=%s, Reason=%s, Message=%s, LastTransitionTime=%v",
			node.Name, notReadyCondition.Status, notReadyCondition.Reason,
			notReadyCondition.Message, notReadyCondition.LastTransitionTime)

		m.notReadyNodes[node.Name] = time.Now()
		klog.Infof("Added/Updated node %s in NotReady nodes list, current count: %d, nodes: %v",
			node.Name, len(m.notReadyNodes), m.getNotReadyNodeNames())
	} else {
		if _, exists := m.notReadyNodes[node.Name]; exists {
			delete(m.notReadyNodes, node.Name)
			klog.Infof("Removed node %s from NotReady nodes list, current count: %d, remaining nodes: %v",
				node.Name, len(m.notReadyNodes), m.getNotReadyNodeNames())
		}
	}
}

// getNotReadyNodeNames returns a list of NotReady node names for logging
func (m *NodeMonitor) getNotReadyNodeNames() []string {
	names := make([]string, 0, len(m.notReadyNodes))
	for name := range m.notReadyNodes {
		names = append(names, name)
	}
	return names
}
