package handlers

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
)

// githubOwner returns the GitHub org/user that owns the registry repo.
// Defaults to "skpm-dev"; override with REGISTRY_GITHUB_OWNER for test environments.
func githubOwner() string {
	if v := os.Getenv("REGISTRY_GITHUB_OWNER"); v != "" {
		return v
	}
	return "skpm-dev"
}

// githubRepo returns the GitHub repository name for registry data.
// Defaults to "registry"; override with REGISTRY_GITHUB_REPO for test environments.
func githubRepo() string {
	if v := os.Getenv("REGISTRY_GITHUB_REPO"); v != "" {
		return v
	}
	return "registry"
}

func computeChecksum(content string) string {
	sum := sha256.Sum256([]byte(content))
	return fmt.Sprintf("sha256:%x", sum)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func marshalIndent(v any) (string, error) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}
