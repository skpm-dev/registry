package handlers

import (
	"testing"
)

func validPublishRequest() publishRequest {
	return publishRequest{
		Manifest: publishManifest{
			Name:        "economy",
			Version:     "1.0.0",
			Description: "A test economy",
		},
		Files: map[string]string{
			"economy.sk": "on spawn:\n  send \"hello\"",
		},
	}
}

func TestValidatePublishRequest_valid(t *testing.T) {
	if err := validatePublishRequest(validPublishRequest()); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestValidatePublishRequest_missingFields(t *testing.T) {
	tests := []struct {
		field  string
		mutate func(*publishRequest)
	}{
		{"name", func(r *publishRequest) { r.Manifest.Name = "" }},
		{"version", func(r *publishRequest) { r.Manifest.Version = "" }},
		{"description", func(r *publishRequest) { r.Manifest.Description = "" }},
		{"files", func(r *publishRequest) { r.Files = nil }},
		{"empty files", func(r *publishRequest) { r.Files = map[string]string{} }},
	}

	for _, tt := range tests {
		t.Run(tt.field, func(t *testing.T) {
			req := validPublishRequest()
			tt.mutate(&req)
			if err := validatePublishRequest(req); err == nil {
				t.Fatalf("expected error for %s, got nil", tt.field)
			}
		})
	}
}

func TestComputeChecksum(t *testing.T) {
	// SHA-256 of "hello" is well-known
	got := computeChecksum("hello")
	want := "sha256:2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824"
	if got != want {
		t.Fatalf("got %s, want %s", got, want)
	}
}

func TestComputeChecksum_emptyString(t *testing.T) {
	got := computeChecksum("")
	want := "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	if got != want {
		t.Fatalf("got %s, want %s", got, want)
	}
}

func TestComputeChecksum_deterministic(t *testing.T) {
	content := "on spawn:\n  send \"hello\""
	if computeChecksum(content) != computeChecksum(content) {
		t.Fatal("checksum is not deterministic")
	}
}
