#!/bin/bash

cd /home/artur/src/github.com/LogicIQ/pvc-chonker

# Generate CA key and cert
openssl genrsa -out ca.key 2048
openssl req -new -x509 -days 365 -key ca.key -out ca.crt -subj "/CN=pvc-chonker-ca"

# Generate server key and cert
openssl genrsa -out tls.key 2048
openssl req -new -key tls.key -out server.csr -subj "/CN=pvc-chonker-webhook-service.pvc-chonker-system.svc"
openssl x509 -req -in server.csr -CA ca.crt -CAkey ca.key -CAcreateserial -out tls.crt -days 365

# Create webhook secret manifest with actual base64 values
cat > config/webhook/webhook-secret.yaml << EOF
apiVersion: v1
kind: Secret
metadata:
  name: pvc-chonker-webhook-server-cert
  namespace: pvc-chonker-system
type: kubernetes.io/tls
data:
  tls.crt: $(base64 -w 0 < tls.crt)
  tls.key: $(base64 -w 0 < tls.key)
  ca.crt: $(base64 -w 0 < ca.crt)
EOF

# Create mutating webhook configuration with actual CA bundle
cat > config/webhook/mutating-webhook-configuration.yaml << EOF
apiVersion: admissionregistration.k8s.io/v1
kind: MutatingAdmissionWebhook
metadata:
  name: pvc-chonker-mutating-webhook-configuration
webhooks:
- name: mpvc.pvc-chonker.io
  clientConfig:
    service:
      name: pvc-chonker-webhook-service
      namespace: pvc-chonker-system
      path: "/mutate-v1-persistentvolumeclaim"
    caBundle: $(base64 -w 0 < ca.crt)
  rules:
  - operations: ["CREATE", "UPDATE"]
    apiGroups: [""]
    apiVersions: ["v1"]
    resources: ["persistentvolumeclaims"]
  admissionReviewVersions: ["v1"]
  sideEffects: None
  failurePolicy: Fail
EOF

# Clean up temp files
rm -f ca.key ca.crt ca.srl tls.key tls.crt server.csr

echo "Webhook manifests created successfully!"