#!/bin/bash

set -e

NAMESPACE=${1:-pvc-chonker-system}
SERVICE_NAME="pvc-chonker-webhook-service"
SECRET_NAME="pvc-chonker-webhook-server-cert"

echo "Creating webhook certificates for namespace: $NAMESPACE"

# Create namespace if it doesn't exist
if ! kubectl create namespace $NAMESPACE --dry-run=client -o yaml | kubectl apply -f -; then
    echo "Error: Failed to create namespace $NAMESPACE"
    exit 1
fi

# Generate CA private key
if ! openssl genrsa -out ca.key 2048; then
    echo "Error: Failed to generate CA private key"
    exit 1
fi

# Generate CA certificate
if ! openssl req -new -x509 -days 365 -key ca.key -out ca.crt -subj "/CN=pvc-chonker-ca"; then
    echo "Error: Failed to generate CA certificate"
    rm -f ca.key
    exit 1
fi

# Generate server private key
if ! openssl genrsa -out tls.key 2048; then
    echo "Error: Failed to generate server private key"
    rm -f ca.key ca.crt
    exit 1
fi

# Generate server certificate signing request
if ! openssl req -new -key tls.key -out server.csr -subj "/CN=$SERVICE_NAME.$NAMESPACE.svc"; then
    echo "Error: Failed to generate server certificate signing request"
    rm -f ca.key ca.crt tls.key
    exit 1
fi

# Generate server certificate
if ! openssl x509 -req -in server.csr -CA ca.crt -CAkey ca.key -CAcreateserial -out tls.crt -days 365 -extensions v3_req -extfile <(cat <<EOF
[v3_req]
basicConstraints = CA:FALSE
keyUsage = nonRepudiation, digitalSignature, keyEncipherment
subjectAltName = @alt_names
[alt_names]
DNS.1 = $SERVICE_NAME
DNS.2 = $SERVICE_NAME.$NAMESPACE
DNS.3 = $SERVICE_NAME.$NAMESPACE.svc
DNS.4 = $SERVICE_NAME.$NAMESPACE.svc.cluster.local
EOF
); then
    echo "Error: Failed to generate server certificate"
    rm -f ca.key ca.crt ca.srl tls.key server.csr
    exit 1
fi

# Create the secret
if ! kubectl create secret tls $SECRET_NAME \
  --cert=tls.crt \
  --key=tls.key \
  --namespace=$NAMESPACE \
  --dry-run=client -o yaml | kubectl apply -f -; then
    echo "Error: Failed to create secret $SECRET_NAME"
    rm -f ca.key ca.crt ca.srl tls.key tls.crt server.csr
    exit 1
fi

# Add CA cert to the secret
if ! kubectl patch secret $SECRET_NAME -n $NAMESPACE -p "{\"data\":{\"ca.crt\":\"$(base64 < ca.crt | tr -d '\n')\"}}"; then
    echo "Error: Failed to patch secret with CA certificate"
    exit 1
fi

# Create mutating webhook configuration
if ! CA_BUNDLE=$(base64 < ca.crt | tr -d '\n'); then
    echo "Error: Failed to encode CA certificate"
    exit 1
fi

if ! cat <<EOF | kubectl apply -f -
apiVersion: admissionregistration.k8s.io/v1
kind: MutatingAdmissionWebhook
metadata:
  name: pvc-chonker-mutating-webhook-configuration
webhooks:
- name: mpvc.pvc-chonker.io
  clientConfig:
    service:
      name: $SERVICE_NAME
      namespace: $NAMESPACE
      path: "/mutate-v1-persistentvolumeclaim"
    caBundle: $CA_BUNDLE
  rules:
  - operations: ["CREATE", "UPDATE"]
    apiGroups: [""]
    apiVersions: ["v1"]
    resources: ["persistentvolumeclaims"]
  admissionReviewVersions: ["v1", "v1beta1"]
  sideEffects: None
  failurePolicy: Fail
EOF
then
    echo "Error: Failed to create MutatingAdmissionWebhook"
    exit 1
fi

# Cleanup temp files
rm -vf ca.key ca.crt ca.srl tls.key tls.crt server.csr

echo "Webhook secret and configuration created successfully!"
echo "Secret: $SECRET_NAME in namespace $NAMESPACE"
echo "MutatingAdmissionWebhook: pvc-chonker-mutating-webhook-configuration"