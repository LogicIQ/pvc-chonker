#!/bin/bash

set -e

NAMESPACE=${1:-pvc-chonker-system}
SERVICE_NAME="pvc-chonker-webhook-service"
SECRET_NAME="pvc-chonker-webhook-server-cert"

echo "Creating webhook certificates for namespace: $NAMESPACE"

# Create namespace if it doesn't exist
kubectl create namespace $NAMESPACE --dry-run=client -o yaml | kubectl apply -f -

# Generate CA private key
openssl genrsa -out ca.key 2048

# Generate CA certificate
openssl req -new -x509 -days 365 -key ca.key -out ca.crt -subj "/CN=pvc-chonker-ca"

# Generate server private key
openssl genrsa -out tls.key 2048

# Generate server certificate signing request
openssl req -new -key tls.key -out server.csr -subj "/CN=$SERVICE_NAME.$NAMESPACE.svc"

# Generate server certificate
openssl x509 -req -in server.csr -CA ca.crt -CAkey ca.key -CAcreateserial -out tls.crt -days 365 -extensions v3_req -extfile <(cat <<EOF
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
)

# Create the secret
kubectl create secret tls $SECRET_NAME \
  --cert=tls.crt \
  --key=tls.key \
  --namespace=$NAMESPACE \
  --dry-run=client -o yaml | kubectl apply -f -

# Add CA cert to the secret
kubectl patch secret $SECRET_NAME -n $NAMESPACE -p "{\"data\":{\"ca.crt\":\"$(base64 -w 0 < ca.crt)\"}}"

# Create mutating webhook configuration
CA_BUNDLE=$(base64 -w 0 < ca.crt)

cat <<EOF | kubectl apply -f -
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

# Cleanup temp files
rm -f ca.key ca.crt ca.srl tls.key tls.crt server.csr

echo "Webhook secret and configuration created successfully!"
echo "Secret: $SECRET_NAME in namespace $NAMESPACE"
echo "MutatingAdmissionWebhook: pvc-chonker-mutating-webhook-configuration"