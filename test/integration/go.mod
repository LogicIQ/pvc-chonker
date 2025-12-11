module github.com/logicIQ/pvc-chonker/test/integration

go 1.25

replace github.com/logicIQ/pvc-chonker => ../..

require (
	github.com/logicIQ/pvc-chonker v0.0.0-00010101000000-000000000000
	github.com/onsi/ginkgo/v2 v2.13.2
	github.com/onsi/gomega v1.30.0
	k8s.io/api v0.32.0
	k8s.io/apimachinery v0.32.0
	k8s.io/client-go v0.32.0
	sigs.k8s.io/controller-runtime v0.20.0
)