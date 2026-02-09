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

	model := c.pickModel(systemPrompt)

	msgs := []openai.ChatCompletionMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: inputJSON},
	}

	req := openai.ChatCompletionRequest{
		Model:    model,
		Messages: msgs,
	}

	// GPT-5 запрещает temperature/top_p/n — не отправляем их
	if !strings.HasPrefix(model, "gpt-5") {
		req.Temperature = 0
	}

	resp, err := c.client.CreateChatCompletion(ctx, req)
	if err != nil {
		log.Println("[ai] OpenAI error:", err)
		return "", err
	}

	if len(resp.Choices) == 0 {
		return "", nil
	}

	raw := resp.Choices[0].Message.Content
	log.Printf("[ai][%s] >>>\n%s\n<<< END\n", model, raw)

	return raw, nil
}

func (c *OpenAIClient) pickModel(systemPrompt string) string {

	switch {
	// ТУПОЙ сбор фактов
	case strings.Contains(systemPrompt, "FACT SELECTOR"):
		return "gpt-4o-mini"

	// УМНЫЙ логик
	case strings.Contains(systemPrompt, "FACT VALIDATOR"):
		return "gpt-5.2"

	// УМНЫЙ писатель
	case strings.Contains(systemPrompt, "ANSWER BUILDER"):
		return "gpt-5.2"

	// УМНЫЙ прокурор
	case strings.Contains(systemPrompt, "ANSWER VALIDATOR"):
		return "gpt-5.2"

	default:
		return "gpt-4o-mini"
	}
}
