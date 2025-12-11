package main

import (
	"flag"
	"os"
	"time"

	"github.com/logicIQ/pvc-chonker/internal/controller"
	"github.com/logicIQ/pvc-chonker/pkg/annotations"
	"github.com/logicIQ/pvc-chonker/pkg/kubelet"

	"context"
	"fmt"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"net/http"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"strings"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	var probeAddr string
	var kubeletURL string
	var watchInterval time.Duration
	var threshold float64
	var increase string
	var cooldown time.Duration
	var minScaleUp string
	var maxSize string
	var dryRun bool
	var logFormat string
	var logLevel string
	var concurrency int

	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "Metrics endpoint address")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "Health probe endpoint address")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false, "Enable leader election")
	flag.StringVar(&kubeletURL, "kubelet-url", "http://localhost:10255", "Kubelet metrics URL")
	flag.DurationVar(&watchInterval, "watch-interval", 5*time.Minute, "Interval for checking PVC usage")
	flag.Float64Var(&threshold, "default-threshold", 0, "Default storage threshold percentage")
	flag.StringVar(&increase, "default-increase", "", "Default expansion amount")
	flag.DurationVar(&cooldown, "default-cooldown", 0, "Default cooldown period")
	flag.StringVar(&minScaleUp, "default-min-scale-up", "", "Default minimum scale-up amount")
	flag.StringVar(&maxSize, "default-max-size", "", "Default maximum size limit")
	flag.BoolVar(&dryRun, "dry-run", false, "Enable dry run mode (no actual PVC modifications)")
	flag.StringVar(&logFormat, "log-format", "json", "Log format: json or console")
	flag.StringVar(&logLevel, "log-level", "info", "Log level: debug, info, warn, error")
	flag.IntVar(&concurrency, "max-parallel", 4, "Maximum parallel PVC operations")

	opts := zap.Options{
		Development: logFormat == "console",
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	setupLog.Info("Logging configuration", "format", logFormat, "level", logLevel)
	if dryRun {
		setupLog.Info("Starting in DRY RUN mode - no PVC modifications will be made")
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		Metrics:                server.Options{BindAddress: metricsAddr},
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "pvc-chonker-leader-election",
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	var minScaleUpQty, maxSizeQty resource.Quantity
	if minScaleUp != "" {
		if qty, err := resource.ParseQuantity(minScaleUp); err == nil {
			minScaleUpQty = qty
		}
	}
	if maxSize != "" {
		if qty, err := resource.ParseQuantity(maxSize); err == nil {
			maxSizeQty = qty
		}
	}

	globalConfig := annotations.NewGlobalConfig(threshold, increase, cooldown, minScaleUpQty, maxSizeQty)
	metricsCollector := kubelet.NewMetricsCollector(kubeletURL)

	pvcController := &controller.PersistentVolumeClaimReconciler{
		Client:           mgr.GetClient(),
		Scheme:           mgr.GetScheme(),
		GlobalConfig:     globalConfig,
		MetricsCollector: metricsCollector,
		WatchInterval:    watchInterval,
		EventRecorder:    mgr.GetEventRecorderFor("pvc-chonker"),
		DryRun:           dryRun,
		MaxParallel:      concurrency,
	}

	// Add the controller as a runnable for periodic reconciliation only
	if err = mgr.Add(pvcController); err != nil {
		setupLog.Error(err, "unable to add controller as runnable")
		os.Exit(1)
	}

	// Add health checks
	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	// Add custom health check for kubelet connectivity
	if err := mgr.AddHealthzCheck("kubelet", func(req *http.Request) error {
		ctx, cancel := context.WithTimeout(req.Context(), 5*time.Second)
		defer cancel()
		_, err := metricsCollector.GetVolumeMetrics(ctx, types.NamespacedName{Name: "test", Namespace: "test"})
		// We expect this to fail for non-existent PVC, but connection should work
		if err != nil && !strings.Contains(err.Error(), "not found") {
			return fmt.Errorf("kubelet connectivity check failed: %w", err)
		}
		return nil
	}); err != nil {
		setupLog.Error(err, "unable to set up kubelet health check")
		os.Exit(1)
	}

	setupLog.Info("starting manager", "dryRun", dryRun, "watchInterval", watchInterval)
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
