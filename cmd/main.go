package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/logicIQ/pvc-chonker/api/v1alpha1"
	"github.com/logicIQ/pvc-chonker/internal/controller"
	"github.com/logicIQ/pvc-chonker/internal/webhook"
	"github.com/logicIQ/pvc-chonker/pkg/annotations"
	"github.com/logicIQ/pvc-chonker/pkg/kubelet"
	"github.com/logicIQ/pvc-chonker/pkg/utils"

	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/runtime"

	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/metrics/server"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
	version  = "dev"
	gitHash  = "unknown"
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(v1alpha1.AddToScheme(scheme))
}

func main() {
	var rootCmd = &cobra.Command{
		Use:   "pvc-chonker",
		Short: "Kubernetes PVC auto-expansion operator",
		Run:   run,
	}

	// Bind flags
	rootCmd.Flags().String("metrics-bind-address", ":8080", "Metrics endpoint address")
	rootCmd.Flags().String("health-probe-bind-address", ":8081", "Health probe endpoint address")
	rootCmd.Flags().Bool("leader-elect", false, "Enable leader election")
	rootCmd.Flags().String("kubelet-url", "", "Custom kubelet metrics URL (for e2e testing, e.g. http://mock-service:8080)")
	rootCmd.Flags().Duration("watch-interval", 5*time.Minute, "Interval for checking PVC usage")
	rootCmd.Flags().Float64("default-threshold", 0, "Default storage threshold percentage")
	rootCmd.Flags().String("default-increase", "", "Default expansion amount")
	rootCmd.Flags().Duration("default-cooldown", 0, "Default cooldown period")
	rootCmd.Flags().String("default-min-scale-up", "", "Default minimum scale-up amount")
	rootCmd.Flags().String("default-max-size", "", "Default maximum size limit")
	rootCmd.Flags().Bool("dry-run", false, "Enable dry run mode (no actual PVC modifications)")
	rootCmd.Flags().String("log-format", "json", "Log format: json or console")
	rootCmd.Flags().String("log-level", "info", "Log level: debug, info, warn, error")
	rootCmd.Flags().Int("max-parallel", 4, "Maximum parallel PVC operations")
	rootCmd.Flags().String("webhook-port", "9443", "Webhook server port")
	rootCmd.Flags().String("webhook-cert-dir", "/tmp/k8s-webhook-server/serving-certs", "Webhook certificate directory")
	rootCmd.Flags().Bool("enable-webhook", false, "Enable admission webhook")

	// Bind viper to flags
	if err := viper.BindPFlags(rootCmd.Flags()); err != nil {
		// Use fmt.Printf since logger isn't set up yet
		fmt.Printf("Error binding flags: %v\n", err)
		os.Exit(1)
	}

	// Set environment variable prefix
	viper.SetEnvPrefix("PVC_CHONKER")
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	viper.AutomaticEnv()

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func run(cmd *cobra.Command, args []string) {
	logFormat := viper.GetString("log-format")
	logLevel := viper.GetString("log-level")
	dryRun := viper.GetBool("dry-run")

	opts := zap.Options{
		Development: logFormat == "console",
	}
	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	setupLog.Info("PVC Chonker starting", "version", version, "gitHash", gitHash)
	setupLog.Info("Logging configuration", "format", utils.SanitizeForLogging(logFormat), "level", utils.SanitizeForLogging(logLevel))
	if dryRun {
		setupLog.Info("Starting in DRY RUN mode - no PVC modifications will be made")
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		Metrics:                server.Options{BindAddress: viper.GetString("metrics-bind-address")},
		HealthProbeBindAddress: viper.GetString("health-probe-bind-address"),
		LeaderElection:         viper.GetBool("leader-elect"),
		LeaderElectionID:       "pvc-chonker-leader-election",
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	var minScaleUpQty, maxSizeQty resource.Quantity
	if minScaleUp := viper.GetString("default-min-scale-up"); minScaleUp != "" {
		if qty, err := resource.ParseQuantity(minScaleUp); err != nil {
			setupLog.Error(err, "invalid default-min-scale-up value", "value", utils.SanitizeForLogging(minScaleUp))
			os.Exit(1)
		} else {
			minScaleUpQty = qty
		}
	}
	if maxSize := viper.GetString("default-max-size"); maxSize != "" {
		if qty, err := resource.ParseQuantity(maxSize); err != nil {
			setupLog.Error(err, "invalid default-max-size value", "value", utils.SanitizeForLogging(maxSize))
			os.Exit(1)
		} else {
			maxSizeQty = qty
		}
	}

	globalConfig := annotations.NewGlobalConfig(
		viper.GetFloat64("default-threshold"),
		viper.GetString("default-increase"),
		viper.GetDuration("default-cooldown"),
		minScaleUpQty,
		maxSizeQty,
	)

	// Use custom kubelet URL if provided via flag or env var (for e2e testing)
	kubeletURL := viper.GetString("kubelet-url")
	if kubeletURL != "" {
		setupLog.Info("Using custom kubelet URL instead of K8s API proxy", "url", utils.SanitizeURL(kubeletURL))
	} else {
		setupLog.Info("Using Kubernetes API proxy for kubelet metrics")
	}
	metricsCollector := kubelet.NewMetricsCollector(kubeletURL)

	// Set Kubernetes clients on metrics collector
	clientset, err := kubernetes.NewForConfig(mgr.GetConfig())
	if err != nil {
		setupLog.Error(err, "unable to create clientset")
		os.Exit(1)
	}
	metricsCollector.SetClient(mgr.GetClient(), clientset)

	pvcController := &controller.PersistentVolumeClaimReconciler{
		Client:           mgr.GetClient(),
		Scheme:           mgr.GetScheme(),
		GlobalConfig:     globalConfig,
		MetricsCollector: metricsCollector,
		WatchInterval:    viper.GetDuration("watch-interval"),
		EventRecorder:    mgr.GetEventRecorderFor("pvc-chonker"),
		DryRun:           dryRun,
		MaxParallel:      viper.GetInt("max-parallel"),
	}

	// Add the controller as a runnable for periodic reconciliation only
	if err = mgr.Add(pvcController); err != nil {
		setupLog.Error(err, "unable to add controller as runnable")
		os.Exit(1)
	}

	// Setup PVCPolicy controller
	policyController := &controller.PVCPolicyReconciler{
		Client:        mgr.GetClient(),
		Scheme:        mgr.GetScheme(),
		EventRecorder: mgr.GetEventRecorderFor("pvc-chonker-policy"),
	}
	if err = policyController.SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create PVCPolicy controller")
		os.Exit(1)
	}

	// Setup PVCGroup controller
	groupController := &controller.PVCGroupReconciler{
		Client:        mgr.GetClient(),
		Scheme:        mgr.GetScheme(),
		EventRecorder: mgr.GetEventRecorderFor("pvc-chonker-group"),
	}
	if err = groupController.SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create PVCGroup controller")
		os.Exit(1)
	}

	// Setup PVCGroup webhook
	if viper.GetBool("enable-webhook") {
		if err = webhook.SetupPVCGroupWebhook(mgr); err != nil {
			setupLog.Error(err, "unable to create PVCGroup webhook")
			os.Exit(1)
		}
		setupLog.Info("PVCGroup webhook enabled")
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

	setupLog.Info("starting manager", "dryRun", dryRun, "watchInterval", utils.SanitizeForLogging(viper.GetDuration("watch-interval").String()))
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
