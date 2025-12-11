package main

import (
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/logicIQ/pvc-chonker/internal/controller"
	"github.com/logicIQ/pvc-chonker/pkg/annotations"
	"github.com/logicIQ/pvc-chonker/pkg/kubelet"

	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/runtime"

	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
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
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
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
	rootCmd.Flags().String("kubelet-url", "http://localhost:10255", "Kubelet metrics URL")
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

	// Bind viper to flags
	viper.BindPFlags(rootCmd.Flags())

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

	setupLog.Info("Logging configuration", "format", logFormat, "level", logLevel)
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
		if qty, err := resource.ParseQuantity(minScaleUp); err == nil {
			minScaleUpQty = qty
		}
	}
	if maxSize := viper.GetString("default-max-size"); maxSize != "" {
		if qty, err := resource.ParseQuantity(maxSize); err == nil {
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
	metricsCollector := kubelet.NewMetricsCollector(viper.GetString("kubelet-url"))

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

	// Add health checks
	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("starting manager", "dryRun", dryRun, "watchInterval", viper.GetDuration("watch-interval"))
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
