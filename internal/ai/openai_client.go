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
		Model:    model,
		Messages: msgs,
	})
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
	case strings.Contains(systemPrompt, "FACT SELECTOR"):
		return "gpt-4o-mini"

	case strings.Contains(systemPrompt, "FACT VALIDATOR"):
		return "gpt-4o-mini"

	case strings.Contains(systemPrompt, "ANSWER BUILDER"):
		return "gpt-5.2"

	case strings.Contains(systemPrompt, "ANSWER VALIDATOR"):
		return "gpt-4o-mini"

	default:
		return "gpt-4o-mini"
	}
}
