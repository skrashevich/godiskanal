package llm

import "testing"

func TestResolveBaseURL(t *testing.T) {
	if got := ResolveBaseURL(""); got != defaultBaseURL {
		t.Errorf("empty = %q, want default", got)
	}
	if got := ResolveBaseURL("https://example.com/v1/"); got != "https://example.com/v1" {
		t.Errorf("trim = %q", got)
	}
}

func TestProviderLabel(t *testing.T) {
	tests := []struct {
		url  string
		want string
	}{
		{"", "OpenAI"},
		{"https://api.openai.com/v1", "OpenAI"},
		{"http://localhost:11434/v1", "Ollama"},
		{"http://127.0.0.1:1234/v1", "LM Studio"},
		{"https://my.openai.azure.com/openai/deployments/x", "Azure OpenAI"},
		{"https://openrouter.ai/api/v1", "OpenRouter"},
		{"https://custom.example.com/v1", "custom.example.com"},
	}
	for _, tc := range tests {
		if got := ProviderLabel(tc.url); got != tc.want {
			t.Errorf("ProviderLabel(%q) = %q, want %q", tc.url, got, tc.want)
		}
	}
}
