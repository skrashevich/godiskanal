package llm

import (
	"strings"
	"testing"
)

func TestNewClient_DefaultURL(t *testing.T) {
	c := NewClient("key", "model", "")
	if c.BaseURL != defaultBaseURL {
		t.Errorf("BaseURL = %q, want %q", c.BaseURL, defaultBaseURL)
	}
}

func TestNewClient_CustomURL(t *testing.T) {
	c := NewClient("key", "model", "https://example.com/v1")
	if c.BaseURL != "https://example.com/v1" {
		t.Errorf("BaseURL = %q, want https://example.com/v1", c.BaseURL)
	}
}

func TestNewClient_TrailingSlashStripped(t *testing.T) {
	c := NewClient("key", "model", "https://example.com/v1/")
	if c.BaseURL != "https://example.com/v1" {
		t.Errorf("BaseURL = %q, trailing slash should be stripped", c.BaseURL)
	}
}

func TestUsageCost(t *testing.T) {
	const M = 1_000_000
	tests := []struct {
		model    string
		prompt   int
		compl    int
		wantCost float64
		wantOK   bool
	}{
		// gpt-4o-mini: $0.150/1M in, $0.600/1M out
		{"gpt-4o-mini", M, 0, 0.150, true},
		{"gpt-4o-mini", 0, M, 0.600, true},
		{"gpt-4o-mini", M, M, 0.750, true},
		{"gpt-4o-mini", 0, 0, 0.000, true},
		// gpt-4o: $2.50/1M in, $10.00/1M out
		{"gpt-4o", M, M, 12.50, true},
		// o3-mini: $1.10/1M in, $4.40/1M out
		{"o3-mini", M, 0, 1.10, true},
		// exact match takes priority
		{"gpt-4o-mini-2024-07-18", M, 0, 0.150, true},
		// unknown model
		{"unknown-model-xyz", 1000, 1000, 0, false},
	}
	for _, tt := range tests {
		u := &Usage{PromptTokens: tt.prompt, CompletionTokens: tt.compl, TotalTokens: tt.prompt + tt.compl}
		got, ok := u.Cost(tt.model)
		if ok != tt.wantOK {
			t.Errorf("Cost(%q): ok=%v, want %v", tt.model, ok, tt.wantOK)
			continue
		}
		if ok && abs64(got-tt.wantCost) > 1e-9 {
			t.Errorf("Cost(%q): cost=%v, want %v", tt.model, got, tt.wantCost)
		}
	}
}

func TestParseSSE_ContentAndUsage(t *testing.T) {
	body := strings.NewReader(
		"data: {\"choices\":[{\"delta\":{\"content\":\"Hello\"}}]}\n" +
			"data: {\"choices\":[{\"delta\":{\"content\":\" World\"}}]}\n" +
			"data: {\"choices\":[],\"usage\":{\"prompt_tokens\":10,\"completion_tokens\":5,\"total_tokens\":15}}\n" +
			"data: [DONE]\n",
	)
	var out strings.Builder
	usage, err := parseSSE(body, &out)
	if err != nil {
		t.Fatalf("parseSSE error: %v", err)
	}
	if !strings.Contains(out.String(), "Hello World") {
		t.Errorf("output = %q, want to contain 'Hello World'", out.String())
	}
	if usage == nil {
		t.Fatal("usage is nil, want non-nil")
	}
	if usage.PromptTokens != 10 {
		t.Errorf("PromptTokens = %d, want 10", usage.PromptTokens)
	}
	if usage.CompletionTokens != 5 {
		t.Errorf("CompletionTokens = %d, want 5", usage.CompletionTokens)
	}
	if usage.TotalTokens != 15 {
		t.Errorf("TotalTokens = %d, want 15", usage.TotalTokens)
	}
}

func TestParseSSE_NoUsage(t *testing.T) {
	body := strings.NewReader(
		"data: {\"choices\":[{\"delta\":{\"content\":\"hi\"}}]}\n" +
			"data: [DONE]\n",
	)
	var out strings.Builder
	usage, err := parseSSE(body, &out)
	if err != nil {
		t.Fatalf("parseSSE error: %v", err)
	}
	if usage != nil {
		t.Errorf("usage = %+v, want nil when API returns no usage", usage)
	}
	if !strings.Contains(out.String(), "hi") {
		t.Errorf("output = %q, want to contain 'hi'", out.String())
	}
}

func TestParseSSE_SkipsNonDataLines(t *testing.T) {
	body := strings.NewReader(
		": keep-alive\n" +
			"\n" +
			"data: {\"choices\":[{\"delta\":{\"content\":\"ok\"}}]}\n" +
			"data: [DONE]\n",
	)
	var out strings.Builder
	_, err := parseSSE(body, &out)
	if err != nil {
		t.Fatalf("parseSSE error: %v", err)
	}
	if !strings.Contains(out.String(), "ok") {
		t.Errorf("output = %q, want to contain 'ok'", out.String())
	}
}

func abs64(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
