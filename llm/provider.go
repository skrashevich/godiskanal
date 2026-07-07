package llm

import (
	"net/url"
	"strings"
)

// ResolveBaseURL returns the effective chat-completions base URL.
func ResolveBaseURL(baseURL string) string {
	baseURL = strings.TrimSpace(baseURL)
	if baseURL == "" {
		return defaultBaseURL
	}
	return strings.TrimRight(baseURL, "/")
}

// ProviderLabel returns a short human-readable name for an API base URL.
func ProviderLabel(baseURL string) string {
	baseURL = ResolveBaseURL(baseURL)
	u, err := url.Parse(baseURL)
	if err != nil {
		return baseURL
	}

	host := strings.ToLower(u.Hostname())
	port := u.Port()

	switch host {
	case "api.openai.com":
		return "OpenAI"
	case "api.groq.com":
		return "Groq"
	case "api.together.xyz":
		return "Together AI"
	case "api.mistral.ai":
		return "Mistral"
	case "openrouter.ai", "api.openrouter.ai":
		return "OpenRouter"
	case "api.deepseek.com":
		return "DeepSeek"
	case "api.fireworks.ai":
		return "Fireworks"
	}

	if strings.Contains(host, "openai.azure.com") {
		return "Azure OpenAI"
	}
	if strings.Contains(host, "anthropic.com") {
		return "Anthropic"
	}
	if strings.Contains(host, "googleapis.com") {
		return "Google"
	}

	if host == "localhost" || host == "127.0.0.1" || host == "::1" {
		switch port {
		case "11434":
			return "Ollama"
		case "1234":
			return "LM Studio"
		case "8080":
			return "Local API"
		default:
			if port != "" {
				return "Local (" + u.Host + ")"
			}
			return "Local API"
		}
	}

	if host != "" {
		return host
	}
	return baseURL
}
