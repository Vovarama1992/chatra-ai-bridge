package chatra

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"time"
)

const chatraBaseURL = "https://app.chatra.io/api/v1"

type ChatraOutbound struct {
	token  string
	client *http.Client
}

func NewChatraOutbound() *ChatraOutbound {
	token := os.Getenv("CHATRA_API_TOKEN")
	if token == "" {
		panic("CHATRA_API_TOKEN not set")
	}

	return &ChatraOutbound{
		token:  token,
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

// Отправка сообщения клиенту (видит клиент)
func (c *ChatraOutbound) SendToChat(
	ctx context.Context,
	chatID string,
	text string,
) error {
	return c.send(ctx, "/chats/"+chatID+"/messages", map[string]any{
		"text": text,
	})
}

// Отправка internal note (видят только операторы)
func (c *ChatraOutbound) SendNote(
	ctx context.Context,
	chatID string,
	text string,
) error {
	return c.send(ctx, "/chats/"+chatID+"/notes", map[string]any{
		"text": text,
	})
}

func (c *ChatraOutbound) send(
	ctx context.Context,
	path string,
	body any,
) error {
	b, err := json.Marshal(body)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		chatraBaseURL+path,
		bytes.NewReader(b),
	)
	if err != nil {
		return err
	}

	// ВАЖНО: Chatra ждёт именно этот хедер
	req.Header.Set("X-Chatra-Access-Token", c.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return errors.New("chatra api error: " + resp.Status)
	}

	return nil
}
