/*
Copyright 2026 The BlanketOps Authors.
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/blanketops/environments-server/pkg/helpers"
)

// CLIRequest represents a request to execute a bops-env CLI command
type CLIRequest struct {
	Command string            `json:"command"` // install, uninstall, dependencies, cluster, secrets, self
	Args    []string          `json:"args"`    // subcommand and arguments
	Env     map[string]string `json:"env,omitempty"`
	Timeout int               `json:"timeout,omitempty"` // seconds
}

// CLIResponse represents the response from a CLI command execution
type CLIResponse struct {
	Success   bool   `json:"success"`
	Output    string `json:"output"`
	Error     string `json:"error,omitempty"`
	ExitCode  int    `json:"exitCode"`
	Duration  string `json:"duration"`
	Command   string `json:"command"`
}

// RegisterCLIRoutes adds CLI management endpoints to the mux
func RegisterCLIRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/cli/exec", HandleCLIExec)
	mux.HandleFunc("GET /api/v1/cli/version", HandleCLIVersion)
	mux.HandleFunc("GET /api/v1/cli/commands", HandleCLICommands)
	mux.HandleFunc("GET /api/v1/cluster/info", HandleClusterInfo)
}

// HandleCLIVersion returns the bops-env CLI version
func HandleCLIVersion(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	output, err := runCLI(ctx, []string{"version"}, nil, 10)
	resp := CLIResponse{
		Success:  err == nil,
		Output:   output,
		Error:    errorString(err),
		ExitCode: exitCode(err),
		Command:  "bops-env version",
	}
	helpers.WriteJSON(w, 200, resp)
}

// HandleCLICommands returns available CLI commands
func HandleCLICommands(w http.ResponseWriter, r *http.Request) {
	commands := map[string]interface{}{
		"operator": map[string]string{
			"install":   "Install the BlanketOps Environments operator",
			"uninstall": "Remove the BlanketOps Environments operator",
		},
		"self": map[string]string{
			"install":   "Fetch and install the latest bops-env CLI binary",
			"uninstall": "Remove a self-installed bops-env CLI binary",
		},
		"dependencies": map[string]string{
			"list":      "List all individually addressable dependencies",
			"install":   "Install the full platform stack, or a specific dependency",
			"uninstall": "Remove the full platform stack, or a specific dependency",
			"status":    "Show install status for all dependencies, or just one",
		},
		"cluster": map[string]string{
			"up":     "Create the named kind cluster if it doesn't exist",
			"down":   "Delete the named kind cluster",
			"status": "Show whether the named kind cluster exists",
		},
		"secrets": map[string]string{
			"bootstrap": "Create all required BlanketOps secrets in the provider",
			"verify":    "List and verify existing BlanketOps secrets in the provider",
		},
		"report": map[string]string{
			"send":      "Send test/benchmark results to Allure server",
			"server":    "Start report receiver server that forwards to Allure",
		},
	}
	helpers.WriteJSON(w, 200, commands)
}

// HandleCLIExec executes a bops-env CLI command
func HandleCLIExec(w http.ResponseWriter, r *http.Request) {
	var req CLIRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		helpers.WriteErr(w, 400, err.Error(), "BadRequest")
		return
	}

	if req.Command == "" {
		helpers.WriteErr(w, 400, "command is required", "BadRequest")
		return
	}

	// Build the full command args
	args := []string{req.Command}
	args = append(args, req.Args...)

	// Validate command is in allowed list
	if !isAllowedCommand(req.Command) {
		helpers.WriteErr(w, 400, fmt.Sprintf("command not allowed: %s", req.Command), "Forbidden")
		return
	}

	timeout := req.Timeout
	if timeout <= 0 {
		timeout = 300 // default 5 minutes
	}
	if timeout > 600 {
		timeout = 600 // max 10 minutes
	}

	slog.Info("Executing CLI command", "command", "bops-env", "args", args)

	start := time.Now()
	output, err := runCLI(r.Context(), args, req.Env, timeout)
	duration := time.Since(start)

	resp := CLIResponse{
		Success:  err == nil,
		Output:   output,
		Error:    errorString(err),
		ExitCode: exitCode(err),
		Duration: duration.String(),
		Command:  "bops-env " + strings.Join(args, " "),
	}

	status := 200
	if !resp.Success {
		status = 500
	}
	helpers.WriteJSON(w, status, resp)
}

// runCLI executes the bops-env binary with the given arguments
func runCLI(ctx context.Context, args []string, env map[string]string, timeoutSeconds int) (string, error) {
	// Find bops-env binary
	binary := "bops-env"
	if _, err := exec.LookPath(binary); err != nil {
		// Try common locations
		paths := []string{
			"/usr/local/bin/bops-env",
			"/home/blanketops/go/bin/bops-env",
			"/home/blanketops/.local/bin/bops-env",
			"./bin/bops-env",
		}
		found := false
		for _, p := range paths {
			if _, err := exec.LookPath(p); err == nil {
				binary = p
				found = true
				break
			}
		}
		if !found {
			return "", fmt.Errorf("bops-env binary not found in PATH")
		}
	}

	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Dir = "/home/blanketops/BlanketOps/sources/environments-cli" // Run from CLI source for embedded assets

	// Set environment
	cmd.Env = append(os.Environ(), "KUBECONFIG=/home/blanketops/.kube/config")
	for k, v := range env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}

	// Capture both stdout and stderr
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Set timeout
	if timeoutSeconds > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(timeoutSeconds)*time.Second)
		defer cancel()
		cmd = exec.CommandContext(ctx, binary, args...)
		cmd.Dir = "/home/blanketops/BlanketOps/sources/environments-cli"
		cmd.Env = append(os.Environ(), "KUBECONFIG=/home/blanketops/.kube/config")
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
	}

	err := cmd.Run()
	output := stdout.String()
	if stderr.Len() > 0 {
		if output != "" {
			output += "\n"
		}
		output += stderr.String()
	}

	return output, err
}

// isAllowedCommand checks if the command is in the allowed list
func isAllowedCommand(cmd string) bool {
	allowed := map[string]bool{
		"install":      true,
		"uninstall":    true,
		"version":      true,
		"self":         true,
		"dependencies": true,
		"cluster":      true,
		"secrets":      true,
		"report":       true,
		"report-server": true,
	}
	return allowed[cmd]
}

func errorString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func exitCode(err error) int {
	if err == nil {
		return 0
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		return exitErr.ExitCode()
	}
	return -1
}

// ClusterInfo represents cluster/operator information
type ClusterInfo struct {
	OperatorVersion string `json:"operatorVersion"`
	ClusterName     string `json:"clusterName"`
	Namespace       string `json:"namespace"`
	Connected       bool   `json:"connected"`
}

// HandleClusterInfo returns cluster and operator information
func HandleClusterInfo(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get operator version from CLI
	versionOutput, err := runCLI(ctx, []string{"version"}, nil, 10)
	operatorVersion := strings.TrimSpace(versionOutput)
	if err != nil || operatorVersion == "" {
		operatorVersion = "unknown"
	}

	// Get cluster name from kubectl config
	clusterName := getClusterName(ctx)

	// Default namespace
	namespace := "blanketops-environments"

	info := ClusterInfo{
		OperatorVersion: operatorVersion,
		ClusterName:     clusterName,
		Namespace:       namespace,
		Connected:       true,
	}

	helpers.WriteJSON(w, 200, info)
}

// getClusterName extracts the current cluster name from kubectl config
func getClusterName(ctx context.Context) string {
	cmd := exec.CommandContext(ctx, "kubectl", "config", "current-context")
	cmd.Env = append(os.Environ(), "KUBECONFIG=/home/blanketops/.kube/config")
	output, err := cmd.Output()
	if err != nil {
		return "unknown"
	}
	name := strings.TrimSpace(string(output))
	// Extract cluster name from ARN if it's an EKS ARN
	if strings.HasPrefix(name, "arn:aws:eks:") {
		parts := strings.Split(name, "/")
		if len(parts) > 1 {
			return parts[len(parts)-1]
		}
	}
	return name
}