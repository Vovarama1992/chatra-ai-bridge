package chatra

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

type ChatraOutbound struct {
	baseURL   string
	publicKey string
	secretKey string
	client    *http.Client
}

func NewChatraOutbound() *ChatraOutbound {
	secret := strings.TrimSpace(os.Getenv("CHATRA_API_TOKEN"))
	if secret == "" {
		panic("CHATRA_API_TOKEN not set (expected SECRET key)")
	}

	return &ChatraOutbound{
		baseURL:   "https://app.chatra.io/api",
		publicKey: "KQN2vdXYigrbe3F36", // PUBLIC key (ChatraID)
		secretKey: secret,              // SECRET key
		client:    &http.Client{Timeout: 10 * time.Second},
	}
}

// ---------- PUBLIC API ----------

// Отправка сообщения клиенту (видит клиент)
func (c *ChatraOutbound) SendToChhat(ctx context.Context, clientID, text string) error {
	return c.send(
		ctx,
		http.MethodPost,
		"/pushedMessages",
		map[string]any{
			"clientId": clientID,
			"text":     text,
		},
	)
}

func (c ChatraOutbound) SendToChat(ctx context.Context, clientID string, text string) error {
	log.Println("[SAFE MODE] BLOCKED SendToChat:", clientID, text)
	return nil
}

// SendNote — заметка для оператора (client info panel), клиент НЕ видит
// Реально поддержано Chatra через PUT /clients/:id → notes
func (c *ChatraOutbound) SendNote(ctx context.Context, clientID, text string) error {
	return c.send(
		ctx,
		http.MethodPut,
		"/clients/"+clientID,
		map[string]any{
			"notes": text,
		},
	)
}

// ---------- INTERNAL ----------

func (c *ChatraOutbound) send(
	ctx context.Context,
	method string,
	path string,
	body any,
) error {
	b, err := json.Marshal(body)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(
		ctx,
		method,
		c.baseURL+path,
		bytes.NewReader(b),
	)
	if err != nil {
		return err
	}

	auth := "Chatra.Simple " + c.publicKey + ":" + c.secretKey

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", auth)

	log.Println("[chatra] METHOD =", method)
	log.Println("[chatra] URL    =", c.baseURL+path)

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return errors.New(
			"chatra api error: " +
				resp.Status +
				" body=" + string(respBody),
		)
	}

	return nil
}
