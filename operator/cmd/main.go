package main

import (
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	v1alpha1 "github.com/opendefensecloud/keycloak-bundle/operator/api/v1alpha1"
	"github.com/opendefensecloud/keycloak-bundle/operator/internal/controller"
	"github.com/opendefensecloud/keycloak-bundle/operator/internal/wrapper"
)

var scheme = runtime.NewScheme()

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(v1alpha1.AddToScheme(scheme))
}

func main() {
	ctrl.SetLogger(zap.New(zap.UseDevMode(getEnv("LOG_LEVEL", "info") == "debug")))
	log := ctrl.Log.WithName("main")

	keycloakURL := getEnv("KEYCLOAK_URL", "http://keycloak:8080")
	keycloakUser := getEnv("KEYCLOAK_USER", "admin")
	
	// Admin password comes from a Secret for STIG compliance
	adminSecretName := getEnv("KEYCLOAK_ADMIN_SECRET_NAME", "keycloak-admin-creds")
	adminSecretKey := getEnv("KEYCLOAK_ADMIN_SECRET_KEY", "password")

	watchNamespace := getEnv("WATCH_NAMESPACE", "")
	checkInterval := getEnvDuration("CHECK_INTERVAL", 30)

	log.Info("starting keycloak-operator",
		"keycloakURL", keycloakURL,
		"watchNamespace", watchNamespace,
		"checkInterval", checkInterval,
	)

	leaderElectionNamespace := watchNamespace
	if leaderElectionNamespace == "" {
		leaderElectionNamespace = "default"
	}

	var namespaceMap map[string]cache.Config
	if watchNamespace != "" {
		namespaceMap = map[string]cache.Config{
			watchNamespace: {},
		}
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme: scheme,
		Cache: cache.Options{
			DefaultNamespaces: namespaceMap,
		},
		LeaderElection:          true,
		LeaderElectionID:        "keycloak-operator-leader",
		LeaderElectionNamespace: leaderElectionNamespace,
		Metrics:                 metricsserver.Options{BindAddress: "0"},
		HealthProbeBindAddress:  ":8081",
	})
	if err != nil {
		log.Error(err, "unable to create manager")
		os.Exit(1)
	}

	if err := mgr.AddHealthzCheck("healthz", func(_ *http.Request) error { return nil }); err != nil {
		log.Error(err, "unable to set up health check")
		os.Exit(1)
	}

	configCLIImage := getEnv("CONFIG_CLI_IMAGE", "")

	runner := &wrapper.JobRunner{
		Client:         mgr.GetClient(),
		APIReader:      mgr.GetAPIReader(),
		URL:            keycloakURL,
		User:           keycloakUser,
		PasswordSecret: &corev1.SecretKeySelector{
			LocalObjectReference: corev1.LocalObjectReference{Name: adminSecretName},
			Key:                  adminSecretKey,
		},
		ConfigCLIImage: configCLIImage,
	}

	if err := (&controller.RealmReconciler{
		Client:        mgr.GetClient(),
		Scheme:        mgr.GetScheme(),
		Runner:        runner,
		Recorder:      mgr.GetEventRecorderFor("keycloak-realm-controller"),
		CheckInterval: checkInterval,
	}).SetupWithManager(mgr); err != nil {
		log.Error(err, "unable to create controller", "controller", "Realm")
		os.Exit(1)
	}

	if err := (&controller.ClientScopeReconciler{
		Client:        mgr.GetClient(),
		Scheme:        mgr.GetScheme(),
		Recorder:      mgr.GetEventRecorderFor("keycloak-clientscope-controller"),
		CheckInterval: checkInterval,
	}).SetupWithManager(mgr); err != nil {
		log.Error(err, "unable to create controller", "controller", "ClientScope")
		os.Exit(1)
	}

	if err := (&controller.GroupReconciler{
		Client:        mgr.GetClient(),
		Scheme:        mgr.GetScheme(),
		Recorder:      mgr.GetEventRecorderFor("keycloak-group-controller"),
		CheckInterval: checkInterval,
	}).SetupWithManager(mgr); err != nil {
		log.Error(err, "unable to create controller", "controller", "Group")
		os.Exit(1)
	}

	if err := (&controller.ClientReconciler{
		Client:        mgr.GetClient(),
		Scheme:        mgr.GetScheme(),
		Recorder:      mgr.GetEventRecorderFor("keycloak-client-controller"),
		CheckInterval: checkInterval,
	}).SetupWithManager(mgr); err != nil {
		log.Error(err, "unable to create controller", "controller", "Client")
		os.Exit(1)
	}

	if err := (&controller.UserReconciler{
		Client:        mgr.GetClient(),
		Scheme:        mgr.GetScheme(),
		Recorder:      mgr.GetEventRecorderFor("keycloak-user-controller"),
		CheckInterval: checkInterval,
	}).SetupWithManager(mgr); err != nil {
		log.Error(err, "unable to create controller", "controller", "User")
		os.Exit(1)
	}

	if err := (&controller.AuthFlowReconciler{
		Client:        mgr.GetClient(),
		Scheme:        mgr.GetScheme(),
		Recorder:      mgr.GetEventRecorderFor("keycloak-authflow-controller"),
		CheckInterval: checkInterval,
	}).SetupWithManager(mgr); err != nil {
		log.Error(err, "unable to create controller", "controller", "AuthFlow")
		os.Exit(1)
	}

	if err := (&controller.IdentityProviderReconciler{
		Client:        mgr.GetClient(),
		Scheme:        mgr.GetScheme(),
		Recorder:      mgr.GetEventRecorderFor("keycloak-identityprovider-controller"),
		CheckInterval: checkInterval,
	}).SetupWithManager(mgr); err != nil {
		log.Error(err, "unable to create controller", "controller", "IdentityProvider")
		os.Exit(1)
	}

	log.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		log.Error(err, "problem running manager")
		os.Exit(1)
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvDuration(key string, fallbackSeconds int) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return time.Duration(fallbackSeconds) * time.Second
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		fmt.Fprintf(os.Stderr, "invalid %s value %q, using default %ds\n", key, v, fallbackSeconds)
		return time.Duration(fallbackSeconds) * time.Second
	}
	return time.Duration(n) * time.Second
}
