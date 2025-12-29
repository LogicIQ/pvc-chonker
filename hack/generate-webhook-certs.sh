#!/bin/bash

cd /home/artur/src/github.com/LogicIQ/pvc-chonker

# Generate new certificate with SANs
openssl req -x509 -newkey rsa:2048 -keyout tls.key -out tls.crt -days 36500 -nodes \
  -subj "/CN=pvc-chonker-webhook-service.pvc-chonker-system.svc" \
  -addext "subjectAltName=DNS:pvc-chonker-webhook-service.pvc-chonker-system.svc,DNS:pvc-chonker-webhook-service.pvc-chonker-system.svc.cluster.local,DNS:pvc-chonker-webhook-service"

# Create webhook secret manifest with actual base64 values
cat > config/webhook/webhook-secret.yaml << EOF
apiVersion: v1
kind: Secret
metadata:
  name: pvc-chonker-webhook-server-cert
  namespace: pvc-chonker-system
  annotations:
    # Security scanner exemptions for test certificates
    security.scan/ignore: "true"
    security.scan/reason: "Test-only hardcoded certificates for e2e testing"
    checkov.io/skip1: CKV_SECRET_6 "Hardcoded secrets for testing"
    kics.io/ignore: "true"
type: kubernetes.io/tls
data:
  tls.crt: $(base64 -w 0 < tls.crt)
  tls.key: $(base64 -w 0 < tls.key)
  ca.crt: $(base64 -w 0 < tls.crt)
EOF

# Create mutating webhook configuration with actual CA bundle
cat > config/webhook/mutating-webhook-configuration.yaml << EOF
apiVersion: admissionregistration.k8s.io/v1
kind: MutatingWebhookConfiguration
metadata:
  name: pvc-chonker-mutating-webhook-configuration
webhooks:
- name: mpvc.pvc-chonker.io
  clientConfig:
    service:
      name: pvc-chonker-webhook-service
      namespace: pvc-chonker-system
      path: "/mutate--v1-persistentvolumeclaim"
    caBundle: $(base64 -w 0 < tls.crt)
  rules:
  - operations: ["CREATE", "UPDATE"]
    apiGroups: [""]
    apiVersions: ["v1"]
    resources: ["persistentvolumeclaims"]
  admissionReviewVersions: ["v1beta1", "v1"]
  sideEffects: None
  failurePolicy: Fail
EOF

# Clean up temp files
rm -f tls.key tls.crt

echo "Webhook manifests created successfully with SANs!"