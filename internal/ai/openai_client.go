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

	log.Printf("\n[AI CALL]\nMODEL: %s\nPROMPT:\n%s\nINPUT:\n%s\n---\n",
		model,
		short(systemPrompt),
		short(inputJSON),
	)

	msgs := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleSystem, Content: systemPrompt},
		{Role: openai.ChatMessageRoleUser, Content: inputJSON},
	}

	req := openai.ChatCompletionRequest{
		Model:    model,
		Messages: msgs,
	}

	resp, err := c.client.CreateChatCompletion(ctx, req)
	if err != nil {
		log.Printf("[AI ERROR][%s] %v\n", model, err)
		return "", err
	}

	if len(resp.Choices) == 0 {
		log.Printf("[AI EMPTY][%s]\n", model)
		return "", nil
	}

	raw := resp.Choices[0].Message.Content

	log.Printf("\n[AI RAW][%s]\n%s\n<<< END RAW\n", model, raw)

	return raw, nil
}

func short(s string) string {
	if len(s) > 400 {
		return s[:400] + "..."
	}
	return s
}

func (c *OpenAIClient) pickModel(systemPrompt string) string {

	switch {
	case strings.Contains(systemPrompt, "FACT SELECTOR"):
		return "gpt-4o-mini"

	case strings.Contains(systemPrompt, "FACT VALIDATOR"):
		return "gpt-5.2"

	case strings.Contains(systemPrompt, "ANSWER BUILDER"):
		return "gpt-5.2"

	case strings.Contains(systemPrompt, "ANSWER VALIDATOR"):
		return "gpt-5.2"

	default:
		return "gpt-4o-mini"
	}
}
