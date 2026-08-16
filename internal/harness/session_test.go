package harness

import "testing"

func TestACPInitializeParamsCarrySelectedProviderAndModel(t *testing.T) {
	params := acpInitializeParams(Options{
		Cwd:      "/workspace/project",
		Provider: "mistral",
		Model:    "mistral-small-latest",
	})

	if got := params["cwd"]; got != "/workspace/project" {
		t.Fatalf("cwd = %v, want %q", got, "/workspace/project")
	}
	if got := params["provider"]; got != "mistral" {
		t.Fatalf("provider = %v, want %q", got, "mistral")
	}
	if got := params["model"]; got != "mistral-small-latest" {
		t.Fatalf("model = %v, want %q", got, "mistral-small-latest")
	}
	if _, ok := params["clientCapabilities"].(map[string]any); !ok {
		t.Fatalf("clientCapabilities = %T, want map[string]any", params["clientCapabilities"])
	}
}
