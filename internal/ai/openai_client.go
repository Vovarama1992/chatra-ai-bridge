package ai

import (
	"context"
	"log"
	"os"

	openai "github.com/sashabaranov/go-openai"
)

type OpenAIClient struct {
	client *openai.Client
	model  string
}

func NewOpenAIClient() *OpenAIClient {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		log.Fatal("OPENAI_API_KEY not set")
	}

	model := os.Getenv("OPENAI_MODEL")
	if model == "" {
		model = openai.GPT4oMini
	}

	return &OpenAIClient{
		client: openai.NewClient(apiKey),
		model:  model,
	}
}

func (c *OpenAIClient) GetReply(
	ctx context.Context,
	history []Message,
) (string, error) {

	// ЖЁСТКИЙ форматный guard — ПОСЛЕДНИМ system
	const jsonGuard = `
Отвечай ТОЛЬКО валидным JSON.
Никакого текста вне JSON.
Формат строго:
{"answer":"строка","confidence":0.0}
Если нарушишь формат — ответ будет отброшен.
`

	msgs := make([]openai.ChatCompletionMessage, 0, len(history)+1)

	for _, m := range history {
		msgs = append(msgs, openai.ChatCompletionMessage{
			Role:    m.Role,
			Content: m.Text,
		})
	}

	// форматный guard — последним system
	msgs = append(msgs, openai.ChatCompletionMessage{
		Role:    "system",
		Content: jsonGuard,
	})

	resp, err := c.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model:    c.model,
		Messages: msgs,
	})
	if err != nil {
		log.Println("[ai] OpenAI error:", err)
		return "", err
	}

	if len(resp.Choices) == 0 {
		log.Println("[ai] empty choices")
		return "", nil
	}

	raw := resp.Choices[0].Message.Content

	// 🔥 КЛЮЧЕВОЕ ЛОГИРОВАНИЕ
	log.Println("[ai] RAW GPT RESPONSE >>>")
	log.Println(raw)
	log.Println("<<< END GPT RESPONSE")

	return raw, nil
}
