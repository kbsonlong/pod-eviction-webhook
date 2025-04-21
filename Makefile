.PHONY: build deploy clean generate-certs

# Build the container image
build:
	docker build -t pod-eviction-protection:latest .

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