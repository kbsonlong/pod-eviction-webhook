.PHONY: build deploy clean generate-certs create-cluster delete-cluster

# Build the container image
build:
	docker buildx build --platform linux/amd64,linux/arm64 -t pod-eviction-protection:latest .

# Deploy the webhook
deploy:
	kubectl apply -f deploy/deployment.yaml
	kubectl apply -f deploy/tls-secret.yaml
	kubectl apply -f deploy/webhook.yaml

# Clean up resources
clean:
	kubectl delete -f deploy/deployment.yaml || true
	kubectl delete -f deploy/tls-secret.yaml || true
	kubectl delete -f deploy/webhook.yaml || true

# Generate TLS certificates
generate-certs:
	chmod +x scripts/generate-cert.sh
	./scripts/generate-cert.sh

# Run tests
test:
	go test ./...

# Run locally
run:
	go run cmd/webhook/main.go --local

# Build for local architecture
build-local:
	docker buildx build  --platform linux/$(shell go env GOARCH) -t pod-eviction-protection:latest . --load

# Create kind test cluster
create-cluster:
	kind create cluster --config kind-config.yaml

# Delete kind test cluster
delete-cluster:
	kind delete cluster --name webhook-test

# Load image to kind cluster
load-image:
	kind load docker-image pod-eviction-protection:latest --name webhook-test

# Setup test environment
setup-test: create-cluster build-local load-image generate-certs deploy 