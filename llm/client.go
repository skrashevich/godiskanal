package llm

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const defaultBaseURL = "https://api.openai.com/v1"

// Client is a simple OpenAI chat completions client.
type Client struct {
	APIKey  string
	Model   string
	BaseURL string
	http    *http.Client
}

// Usage contains token counts returned by the API.
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// Cost calculates the USD cost for this usage given a model name.
func (u *Usage) Cost(model string) (float64, bool) {
	// [input per 1M tokens, output per 1M tokens] in USD
	pricing := map[string][2]float64{
		"gpt-4o-mini":            {0.150, 0.600},
		"gpt-4o-mini-2024-07-18": {0.150, 0.600},
		"gpt-4o":                 {2.50, 10.00},
		"gpt-4o-2024-11-20":      {2.50, 10.00},
		"gpt-4o-2024-08-06":      {2.50, 10.00},
		"gpt-4-turbo":            {10.00, 30.00},
		"gpt-4-turbo-preview":    {10.00, 30.00},
		"gpt-4":                  {30.00, 60.00},
		"gpt-3.5-turbo":          {0.50, 1.50},
		"o1":                     {15.00, 60.00},
		"o1-mini":                {3.00, 12.00},
		"o3-mini":                {1.10, 4.40},
		"o3":                     {10.00, 40.00},
	}

	// Try exact match first, then check prefix (e.g. "gpt-4o-mini-...")
	p, ok := pricing[model]
	if !ok {
		for key, val := range pricing {
			if strings.HasPrefix(model, key) {
				p = val
				ok = true
				break
			}
		}
	}
	if !ok {
		return 0, false
	}

	cost := float64(u.PromptTokens)/1_000_000*p[0] +
		float64(u.CompletionTokens)/1_000_000*p[1]
	return cost, true
}

// NewClient creates a new LLM client. If baseURL is empty, the default OpenAI endpoint is used.
func NewClient(apiKey, model, baseURL string) *Client {
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	baseURL = strings.TrimRight(baseURL, "/")
	return &Client{
		APIKey:  apiKey,
		Model:   model,
		BaseURL: baseURL,
		http:    &http.Client{Timeout: 120 * time.Second},
	}
}

type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type streamOptions struct {
	IncludeUsage bool `json:"include_usage"`
}

type chatRequest struct {
	Model         string         `json:"model"`
	Messages      []message      `json:"messages"`
	Stream        bool           `json:"stream,omitempty"`
	StreamOptions *streamOptions `json:"stream_options,omitempty"` // only for streaming
}

// Describe sends a single-turn completion request and returns the full response.
// Used by the TUI browser to explain what a directory/file is.
func (c *Client) Describe(userPrompt string) (string, error) {
	req := chatRequest{
		Model: c.Model,
		Messages: []message{
			{
				Role: "system",
				Content: `Ты эксперт по macOS. Кратко (2-4 предложения) объясни что это за путь
(директория или файл) и безопасно ли его удалить для освобождения места.
Отвечай на русском языке. Если путь явно системный или критически важный — предупреди об этом.`,
			},
			{Role: "user", Content: userPrompt},
		},
		Stream: false,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequest("POST", c.BaseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.APIKey)

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("HTTP request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("API error %d: %s", resp.StatusCode, string(b))
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}
	if len(result.Choices) == 0 {
		return "", fmt.Errorf("пустой ответ от API")
	}
	return result.Choices[0].Message.Content, nil
}

// StreamAnalysis sends disk analysis data to the LLM and streams the response to out.
// Returns Usage if the provider returned it, or nil.
func (c *Client) StreamAnalysis(prompt string, out io.Writer) (*Usage, error) {
	req := chatRequest{
		Model: c.Model,
		Messages: []message{
			{
				Role: "system",
				Content: `Ты эксперт по macOS, помогающий пользователям освободить место на диске.
Анализируй данные об использовании диска и давай конкретные, actionable рекомендации.
Приоритизируй рекомендации по потенциальному объёму освобождаемого места.
Используй markdown: заголовки, жирный текст, списки. Отвечай на русском языке.
Будь конкретен: указывай точные команды и пути для очистки.`,
			},
			{
				Role:    "user",
				Content: prompt,
			},
		},
		Stream:        true,
		StreamOptions: &streamOptions{IncludeUsage: true},
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequest("POST", c.BaseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.APIKey)

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("HTTP request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(b))
	}

	return parseSSE(resp.Body, out)
}

// parseSSE reads Server-Sent Events, writes content to out, and returns usage if present.
func parseSSE(r io.Reader, out io.Writer) (*Usage, error) {
	var usage *Usage

	scanner := bufio.NewScanner(r)
	// Increase buffer for large chunks
	scanner.Buffer(make([]byte, 64*1024), 64*1024)

	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := line[6:] // strip "data: "
		if data == "[DONE]" {
			break
		}

		var chunk struct {
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
			} `json:"choices"`
			Usage *Usage `json:"usage"`
		}
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}

		// Capture usage from the final summary chunk (choices is empty)
		if chunk.Usage != nil && chunk.Usage.TotalTokens > 0 {
			usage = chunk.Usage
		}

		if len(chunk.Choices) > 0 {
			fmt.Fprint(out, chunk.Choices[0].Delta.Content)
		}
	}

	fmt.Fprintln(out) // final newline
	return usage, scanner.Err()
}
