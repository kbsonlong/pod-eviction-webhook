#!/bin/bash

set -e

# Create temporary directory for certificates
CERT_DIR=$(mktemp -d)
echo "Creating certificates in ${CERT_DIR}"

# Generate CA certificate
openssl genrsa -out ${CERT_DIR}/ca.key 2048
openssl req -x509 -new -nodes -key ${CERT_DIR}/ca.key -subj "/CN=pod-eviction-protection-ca" -days 3650 -out ${CERT_DIR}/ca.crt

# Generate server certificate
openssl genrsa -out ${CERT_DIR}/tls.key 2048
openssl req -new -key ${CERT_DIR}/tls.key -subj "/CN=pod-eviction-protection.default.svc" -out ${CERT_DIR}/tls.csr
openssl x509 -req -in ${CERT_DIR}/tls.csr -CA ${CERT_DIR}/ca.crt -CAkey ${CERT_DIR}/ca.key -CAcreateserial -out ${CERT_DIR}/tls.crt -days 3650

# Create Kubernetes secret
kubectl create secret tls pod-eviction-protection-tls \
  --cert=${CERT_DIR}/tls.crt \
  --key=${CERT_DIR}/tls.key \
  --dry-run=client -o yaml > deploy/tls-secret.yaml

# Get CA bundle for webhook configuration
CA_BUNDLE=$(cat ${CERT_DIR}/ca.crt | base64 | tr -d '\n')
sed "s/\${CA_BUNDLE}/${CA_BUNDLE}/g" deploy/webhook.yaml.template > deploy/webhook.yaml

echo "Certificates generated successfully"
echo "CA bundle: ${CA_BUNDLE}"
echo "Please apply the following files:"
echo "1. deploy/tls-secret.yaml"
echo "2. deploy/webhook.yaml" 