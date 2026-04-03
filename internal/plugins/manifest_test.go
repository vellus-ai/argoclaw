package plugins_test

import (
	"testing"

	"github.com/vellus-ai/argoclaw/internal/plugins"
)

func TestParseManifest_ValidYAML(t *testing.T) {
	yaml := `
name: prompt-vault
display_name: "Prompt Vault"
version: "0.1.0"
description: "Versioning for prompts"
author: "Vellus AI"
runtime:
  transport: stdio
  command: "./server"
permissions:
  tools:
    provide:
      - name: vault_prompt_create
        description: "Create a prompt"
  data:
    write:
      - "plugin:prompt-vault-*"
    read:
      - "plugin:prompt-vault-*"
`
	m, err := plugins.ParseManifest([]byte(yaml))
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if m.Name != "prompt-vault" {
		t.Errorf("expected name 'prompt-vault', got %q", m.Name)
	}
	if m.DisplayName != "Prompt Vault" {
		t.Errorf("expected display_name 'Prompt Vault', got %q", m.DisplayName)
	}
	if m.Version != "0.1.0" {
		t.Errorf("expected version '0.1.0', got %q", m.Version)
	}
	if m.Runtime.Transport != "stdio" {
		t.Errorf("expected transport 'stdio', got %q", m.Runtime.Transport)
	}
	if len(m.Permissions.Tools.Provide) != 1 {
		t.Errorf("expected 1 tool, got %d", len(m.Permissions.Tools.Provide))
	}
}

func TestParseManifest_MissingName(t *testing.T) {
	yaml := `
display_name: "No Name Plugin"
version: "1.0.0"
runtime:
  transport: stdio
`
	_, err := plugins.ParseManifest([]byte(yaml))
	if err == nil {
		t.Fatal("expected error for missing name, got nil")
	}
}

func TestParseManifest_MissingVersion(t *testing.T) {
	yaml := `
name: my-plugin
display_name: "My Plugin"
runtime:
  transport: stdio
`
	_, err := plugins.ParseManifest([]byte(yaml))
	if err == nil {
		t.Fatal("expected error for missing version, got nil")
	}
}

func TestParseManifest_InvalidSemver(t *testing.T) {
	yaml := `
name: my-plugin
display_name: "My Plugin"
version: "not-a-semver"
runtime:
  transport: stdio
`
	_, err := plugins.ParseManifest([]byte(yaml))
	if err == nil {
		t.Fatal("expected error for invalid semver, got nil")
	}
}

func TestParseManifest_InvalidTransport(t *testing.T) {
	yaml := `
name: my-plugin
display_name: "My Plugin"
version: "1.0.0"
runtime:
  transport: grpc
`
	_, err := plugins.ParseManifest([]byte(yaml))
	if err == nil {
		t.Fatal("expected error for invalid transport, got nil")
	}
}

func TestParseManifest_InvalidNameFormat(t *testing.T) {
	tests := []struct {
		name string
		yaml string
	}{
		{
			name: "uppercase letters",
			yaml: "name: MyPlugin\ndisplay_name: x\nversion: \"1.0.0\"\nruntime:\n  transport: stdio\n",
		},
		{
			name: "spaces",
			yaml: "name: my plugin\ndisplay_name: x\nversion: \"1.0.0\"\nruntime:\n  transport: stdio\n",
		},
		{
			name: "leading hyphen",
			yaml: "name: -myplugin\ndisplay_name: x\nversion: \"1.0.0\"\nruntime:\n  transport: stdio\n",
		},
		{
			name: "too short",
			yaml: "name: ab\ndisplay_name: x\nversion: \"1.0.0\"\nruntime:\n  transport: stdio\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := plugins.ParseManifest([]byte(tt.yaml))
			if err == nil {
				t.Fatalf("expected error for invalid name %q, got nil", tt.name)
			}
		})
	}
}

func TestParseManifest_InvalidYAML(t *testing.T) {
	_, err := plugins.ParseManifest([]byte(":::invalid yaml:::"))
	if err == nil {
		t.Fatal("expected error for malformed YAML, got nil")
	}
}

func TestParseManifest_MissingRuntime(t *testing.T) {
	yaml := `
name: my-plugin
display_name: "My Plugin"
version: "1.0.0"
`
	_, err := plugins.ParseManifest([]byte(yaml))
	if err == nil {
		t.Fatal("expected error for missing runtime, got nil")
	}
}

func TestParseManifest_ValidTransports(t *testing.T) {
	transports := []string{"stdio", "sse", "streamable-http", "sidecar"}
	for _, transport := range transports {
		t.Run(transport, func(t *testing.T) {
			yaml := "name: my-plugin\ndisplay_name: \"My Plugin\"\nversion: \"1.0.0\"\nruntime:\n  transport: " + transport + "\n"
			_, err := plugins.ParseManifest([]byte(yaml))
			if err != nil {
				t.Fatalf("expected transport %q to be valid, got: %v", transport, err)
			}
		})
	}
}
