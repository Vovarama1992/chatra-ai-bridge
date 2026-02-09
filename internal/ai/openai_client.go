package ai

import (
	"context"
	"log"
	"os"
	"strings"

	openai "github.com/sashabaranov/go-openai"
)

type OpenAIClient struct {
	client *openai.Client
}

func NewOpenAIClient() *OpenAIClient {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		log.Fatal("OPENAI_API_KEY not set")
	}

	return &OpenAIClient{
		client: openai.NewClient(apiKey),
	}
}

func (c *OpenAIClient) GetReply(
	ctx context.Context,
	systemPrompt string,
	inputJSON string,
) (string, error) {

	model, temperature := c.pickModelAndTemp(systemPrompt)

	msgs := []openai.ChatCompletionMessage{
		{
			Role:    "system",
			Content: systemPrompt,
		},
		{
			Role:    "user",
			Content: inputJSON,
		},
	}

	resp, err := c.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model:       model,
		Messages:    msgs,
		Temperature: temperature,
	})
	if err != nil {
		log.Println("[ai] OpenAI error:", err)
		return "", err
	}

	if len(resp.Choices) == 0 {
		return "", nil
	}

	raw := resp.Choices[0].Message.Content

	log.Printf("[ai][%s][t=%.1f] >>>\n%s\n<<< END\n", model, temperature, raw)

	return raw, nil
}

func (c *OpenAIClient) pickModelAndTemp(systemPrompt string) (string, float32) {

	switch {
	// ---- ТУПОЙ СБОРЩИК ФАКТОВ ----
	case strings.Contains(systemPrompt, "FACT SELECTOR"):
		return "gpt-4o-mini", 0.0

	// ---- УМНЫЙ ЛОГИК ----
	case strings.Contains(systemPrompt, "FACT VALIDATOR"):
		return "gpt-5.2", 0.0

	// ---- УМНЫЙ ПИСАТЕЛЬ ----
	case strings.Contains(systemPrompt, "ANSWER BUILDER"):
		return "gpt-5.2", 0.3

	// ---- УМНЫЙ ПРОКУРОР ----
	case strings.Contains(systemPrompt, "ANSWER VALIDATOR"):
		return "gpt-5.2", 0.0

	default:
		return "gpt-4o-mini", 0.0
	}
}
