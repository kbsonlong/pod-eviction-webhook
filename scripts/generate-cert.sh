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

# Create CSR config file with SANs
cat > ${CERT_DIR}/csr.conf << EOF
[req]
req_extensions = v3_req
distinguished_name = req_distinguished_name
[req_distinguished_name]
[v3_req]
basicConstraints = CA:FALSE
keyUsage = nonRepudiation, digitalSignature, keyEncipherment
extendedKeyUsage = serverAuth
subjectAltName = @alt_names
[alt_names]
DNS.1 = pod-eviction-protection
DNS.2 = pod-eviction-protection.default
DNS.3 = pod-eviction-protection.default.svc
DNS.4 = pod-eviction-protection.default.svc.cluster.local
EOF

# Generate CSR with SANs
openssl req -new -key ${CERT_DIR}/tls.key -subj "/CN=pod-eviction-protection.default.svc" \
    -config ${CERT_DIR}/csr.conf -out ${CERT_DIR}/tls.csr

# Sign the certificate with CA
openssl x509 -req -in ${CERT_DIR}/tls.csr -CA ${CERT_DIR}/ca.crt -CAkey ${CERT_DIR}/ca.key \
    -CAcreateserial -out ${CERT_DIR}/tls.crt -days 3650 \
    -extensions v3_req -extfile ${CERT_DIR}/csr.conf

# Create Kubernetes secret
kubectl create secret tls webhook-server-cert \
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