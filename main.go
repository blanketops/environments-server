package main

import (
	"log/slog"
	"net/http"
	"os"
	"path/filepath"

	"github.com/blanketops/environments-server/handlers"
	"github.com/blanketops/environments-server/pkg/store"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8082"
	}

	// Set log level to debug
	slog.SetLogLoggerLevel(slog.LevelDebug)

	var config *rest.Config
	var err error

	// Try in-cluster config first
	config, err = rest.InClusterConfig()
	if err != nil {
		// Fall back to kubeconfig
		kubeconfig := os.Getenv("KUBECONFIG")
		if kubeconfig == "" {
			home, _ := os.UserHomeDir()
			kubeconfig = filepath.Join(home, ".kube", "config")
		}
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			slog.Error("failed to get k8s config", "err", err)
			os.Exit(1)
		}
	}

	var s store.Store
	if os.Getenv("MOCK") == "1" || os.Getenv("ENVIRONMENTS_MOCK") == "1" {
		s, err = store.NewMockStore()
		if err != nil {
			slog.Error("failed to create mock store", "err", err)
			os.Exit(1)
		}
		slog.Info("environments server running in mock mode (no cluster required)")
	} else {
		k8s, err := store.NewK8sStore(config)
		if err != nil {
			slog.Error("failed to create k8s store", "err", err)
			os.Exit(1)
		}
		s = k8s
	}

	mux := handlers.SetupRoutes(s)

	slog.Info("blanketops environments server", "addr", ":"+port)
	if err := http.ListenAndServe(":"+port, handlers.Cors(mux)); err != nil {
		slog.Error("server error", "err", err)
		os.Exit(1)
	}
}