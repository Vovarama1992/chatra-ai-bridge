package chatra

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
)

type Handler struct {
	svc Service
}

func NewHandler(svc Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) HandleWebhook(w http.ResponseWriter, r *http.Request) {
	log.Println("[chatra] webhook hit")

	log.Println("[chatra HEADERS]")
	for k, v := range r.Header {
		log.Printf("%s: %v\n", k, v)
	}

	body, _ := io.ReadAll(r.Body)
	log.Printf("[chatra RAW BODY]\n%s\n", body)

	// вернуть body обратно для Decode
	r.Body = io.NopCloser(bytes.NewBuffer(body))

	var payload struct {
		EventName string `json:"eventName"`
		Messages  []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"messages"`
		Client struct {
			ChatID string         `json:"chatId"`
			ID     string         `json:"id"`
			Info   map[string]any `json:"info"`
			Int    map[string]any `json:"integrationData"`
		} `json:"client"`
	}

	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		log.Println("[chatra] decode error:", err)
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	log.Printf(
		"[chatra] event=%s chatId=%s clientId=%s messages=%d",
		payload.EventName,
		payload.Client.ChatID,
		payload.Client.ID,
		len(payload.Messages),
	)

	// ACK СРАЗУ
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))

	if payload.EventName != "chatFragment" {
		log.Println("[chatra] skip non chatFragment")
		return
	}

	p := payload

	// ВСЯ ОБРАБОТКА — В ФОНЕ
	// ВСЯ ОБРАБОТКА — В ФОНЕ
	go func() {
		ctx := context.Background()

		for i, m := range p.Messages {
			log.Printf("[chatra] msg[%d] type=%s text=%q", i, m.Type, m.Text)

			if m.Text == "" {
				continue
			}

			switch m.Type {

			case "client":
				msg := &Message{
					ChatID:            p.Client.ChatID,
					Sender:            SenderClient,
					Text:              m.Text,
					ClientID:          &p.Client.ID,
					ClientInfo:        p.Client.Info,
					ClientIntegration: p.Client.Int,
				}

				log.Println("[chatra] -> HandleIncoming start")

				if err := h.svc.HandleIncoming(ctx, msg); err != nil {
					log.Println("[chatra] HandleIncoming error:", err)
				}

				log.Println("[chatra] -> HandleIncoming done")

			case "agent":
				if err := h.svc.SaveOnly(ctx, &Message{
					ChatID:   p.Client.ChatID,
					Sender:   SenderSupporter,
					Text:     m.Text,
					ClientID: &p.Client.ID,
				}); err != nil {
					log.Println("[chatra] Save agent message error:", err)
				}

			case "system":
				// игнорируем системные сообщения
				continue
			}
		}
	}()

	log.Println("[chatra] webhook ACK sent")
}
