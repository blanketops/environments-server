package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

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

	slog.SetLogLoggerLevel(slog.LevelDebug)

	var config *rest.Config
	var err error

	config, err = rest.InClusterConfig()
	if err != nil {
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

	server := &http.Server{
		Addr:              ":" + port,
		Handler:           handlers.Cors(mux),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	// Graceful shutdown on SIGTERM/SIGINT (Knative sends SIGTERM)
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGTERM, syscall.SIGINT)

	go func() {
		slog.Info("environments server listening", "addr", ":"+port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "err", err)
			os.Exit(1)
		}
	}()

	<-stop
	slog.Info("shutting down server")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		slog.Error("server forced to shutdown", "err", err)
		os.Exit(1)
	}
	slog.Info("server exited")
}