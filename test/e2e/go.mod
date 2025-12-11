module github.com/logicIQ/pvc-chonker/test/e2e

go 1.21

replace github.com/logicIQ/pvc-chonker => ../..

require (
	github.com/onsi/ginkgo/v2 v2.13.2
	github.com/onsi/gomega v1.30.0
	k8s.io/api v0.32.0
	k8s.io/apimachinery v0.32.0
	k8s.io/client-go v0.32.0
	sigs.k8s.io/controller-runtime v0.20.0
)