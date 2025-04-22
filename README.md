# Kubernetes Pod Eviction Protection Webhook

## 项目概述

这是一个Kubernetes Admission Webhook，用于在集群出现多个工作节点NotReady状态时，保护Pod不被驱逐。主要功能包括：

1. 监控集群工作节点状态
2. 在5分钟内检测到3台及以上Worknodes出现NotReady状态时，自动开启Pod驱逐拦截
3. 拦截Update事件，避免更新Pod状态和添加`deletionGracePeriodSeconds`和`deletionTimestamp`等元数据
4. 只拦截NotReady节点上的Pod驱逐操作，其他正常节点上的Pod允许正常驱逐
5. 提供配置接口，允许动态调整监控参数

## 架构设计

### 核心组件

1. **Webhook Server**: 处理Kubernetes Admission请求
2. **Node Monitor**: 监控节点状态变化
3. **Eviction Interceptor**: 拦截Pod驱逐请求
4. **Configuration Manager**: 管理Webhook配置

### 技术栈

- Go 1.20+
- Kubernetes Client-go
- Gin Web Framework
- Prometheus Metrics

## 配置说明

### 环境变量

- `WEBHOOK_PORT`: Webhook服务端口，默认8443
- `CERT_DIR`: TLS证书目录，默认/tmp/k8s-webhook-server/serving-certs
- `NODE_NOTREADY_THRESHOLD`: 触发拦截的NotReady节点数量阈值，默认3
- `NODE_NOTREADY_WINDOW`: 检测时间窗口，默认5分钟

### 部署配置

```yaml
apiVersion: admissionregistration.k8s.io/v1
kind: ValidatingWebhookConfiguration
metadata:
  name: pod-eviction-protection
webhooks:
- name: pod-eviction-protection.webhook.io
  clientConfig:
    service:
      name: pod-eviction-protection
      namespace: default
      path: "/validate"
    caBundle: ${CA_BUNDLE}
  rules:
  - operations: ["DELETE","UPDATE"]
    apiGroups: [""]
    apiVersions: ["v1"]
    resources: ["pods"]
  failurePolicy: Fail
  sideEffects: None
  admissionReviewVersions: ["v1"]
```

## 监控指标

- `node_notready_count`: 当前NotReady节点数量
- `eviction_intercepted_total`: 拦截的驱逐请求总数
- `eviction_allowed_total`: 允许的驱逐请求总数

## 开发指南

### 本地开发

1. 安装依赖：
```bash
go mod tidy
```

2. 运行测试：
```bash
go test ./...
```

3. 本地运行：
```bash
# 使用本地kubeconfig运行
go run cmd/webhook/main.go --local

# 或者使用环境变量覆盖默认配置
WEBHOOK_PORT=8080 go run cmd/webhook/main.go --local
```

4. 本地开发注意事项：
- 本地开发模式下，webhook使用HTTP而不是HTTPS
- 确保本地kubeconfig文件存在且配置正确（~/.kube/config）
- 本地开发时，webhook会连接到本地配置的Kubernetes集群
- 可以通过环境变量覆盖默认配置参数

### 构建部署

1. 构建镜像：
```bash
docker build -t pod-eviction-protection:latest .
```

2. 部署到集群：
```bash
kubectl apply -f deploy/
```

## 性能考虑

1. 使用缓存减少API Server请求
2. 异步处理节点状态更新
3. 使用连接池优化API Server通信
4. 实现请求限流保护

## 安全考虑

1. 使用TLS证书认证
2. 实现RBAC权限控制
3. 限制Webhook访问范围
4. 实现请求验证和审计日志

## 维护指南

1. 定期检查日志和监控指标
2. 及时更新依赖包版本
3. 定期备份配置数据
4. 制定应急预案