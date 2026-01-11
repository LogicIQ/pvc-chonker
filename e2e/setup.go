package e2e

import (
	"context"
	"io"
	"math/rand"
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"

	pvcchonkerv1alpha1 "github.com/LogicIQ/pvc-chonker/api/v1alpha1"
)

const testNamespace = "pvc-chonker-system"

var clientset *kubernetes.Clientset
var k8sClient client.Client

func init() {
	cfg, err := config.GetConfig()
	if err != nil {
		panic(err)
	}
	clientset, err = kubernetes.NewForConfig(cfg)
	if err != nil {
		panic(err)
	}
	
	scheme := runtime.NewScheme()
	if err := pvcchonkerv1alpha1.AddToScheme(scheme); err != nil {
		panic(err)
	}
	if err := corev1.AddToScheme(scheme); err != nil {
		panic(err)
	}
	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme})
	if err != nil {
		panic(err)
	}
}

func getK8sClient(t *testing.T) client.Client {
	if k8sClient == nil {
		t.Fatal("k8sClient is not initialized")
	}
	return k8sClient
}

func generateRandomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	rand.Seed(time.Now().UnixNano())
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return string(b)
}

func TestMain(m *testing.M) {
	m.Run()
}

func waitForPod(t *testing.T, podName, namespace string) {
	t.Logf("Waiting for pod %s in namespace %s to be ready...", podName, namespace)
	if clientset == nil {
		t.Fatal("clientset is not initialized")
	}
	ctx := context.Background()
	err := wait.PollUntilContextTimeout(ctx, 2*time.Second, 60*time.Second, true, func(ctx context.Context) (bool, error) {
		pod, err := clientset.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
		if err != nil {
			t.Logf("Pod %s not found yet: %v", podName, err)
			return false, nil
		}
		if pod.Status.Phase == corev1.PodRunning {
			for _, condition := range pod.Status.Conditions {
				if condition.Type == corev1.PodReady && condition.Status == corev1.ConditionTrue {
					return true, nil
				}
			}
		}
		t.Logf("Pod %s status: %s", podName, pod.Status.Phase)
		return false, nil
	})
	if err != nil {
		t.Fatalf("Pod %s not ready: %v", podName, err)
	}
}

func waitForOperator(t *testing.T) {
	t.Log("Waiting for operator to be ready...")
	if clientset == nil {
		t.Fatal("clientset is not initialized")
	}
	ctx := context.Background()
	err := wait.PollUntilContextTimeout(ctx, 3*time.Second, 30*time.Second, true, func(ctx context.Context) (bool, error) {
		pods, err := clientset.CoreV1().Pods(testNamespace).List(ctx, metav1.ListOptions{
			LabelSelector: "control-plane=controller-manager",
		})
		if err != nil {
			return false, err
		}
		if len(pods.Items) == 0 {
			return false, nil
		}
		for _, pod := range pods.Items {
			if pod.Status.Phase != corev1.PodRunning {
				return false, nil
			}
		}
		return true, nil
	})
	if err != nil {
		t.Fatalf("Operator not ready: %v", err)
	}
}

func getOperatorLogs(t *testing.T) string {
	ctx := context.Background()
	
	pods, err := clientset.CoreV1().Pods(testNamespace).List(ctx, metav1.ListOptions{
		LabelSelector: "control-plane=controller-manager",
	})
	if err != nil {
		t.Fatalf("Failed to get operator pods: %v", err)
	}
	
	if len(pods.Items) == 0 {
		t.Fatal("No operator pods found")
	}
	
	podName := pods.Items[0].Name
	req := clientset.CoreV1().Pods(testNamespace).GetLogs(podName, &corev1.PodLogOptions{
		TailLines: int64Ptr(2),
	})
	
	logs, err := req.Stream(ctx)
	if err != nil {
		t.Fatalf("Failed to get logs: %v", err)
	}
	defer logs.Close()
	
	logData, err := io.ReadAll(logs)
	if err != nil {
		t.Fatalf("Failed to read logs: %v", sanitizeError(err))
	}
	
	return string(logData)
}

func int64Ptr(i int64) *int64 {
	return &i
}

func stringPtr(s string) *string {
	return &s
}

// sanitizeError sanitizes error messages for safe logging
func sanitizeError(err error) string {
	if err == nil {
		return ""
	}
	// Remove all control characters including newlines, carriage returns, tabs, and ANSI escape sequences
	var result strings.Builder
	for _, r := range err.Error() {
		// Only allow printable ASCII characters and spaces
		if r >= 32 && r <= 126 {
			result.WriteRune(r)
		}
	}
	return result.String()
}