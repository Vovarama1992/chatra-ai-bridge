package chatra

import (
	"encoding/json"
	"net/http"
)

type Handler struct {
	svc Service
}

func NewHandler(svc Service) *Handler {
	return &Handler{svc: svc}
}

// HandleWebhook — вход от Chatra
func (h *Handler) HandleWebhook(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		EventName string `json:"eventName"`
		Messages  []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"messages"`
		Client struct {
			ChatID string `json:"chatId"`
			ID     string `json:"id"`
		} `json:"client"`
	}

	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	// Нас интересует только chatFragment
	if payload.EventName != "chatFragment" {
		w.WriteHeader(http.StatusOK)
		return
	}

	// Берём все сообщения клиента
	for _, m := range payload.Messages {
		if m.Type != "client" || m.Text == "" {
			continue
		}

		msg := &Message{
			ChatID:   payload.Client.ChatID,
			Sender:   SenderClient,
			Text:     m.Text,
			ClientID: &payload.Client.ID,
			// SupporterID пока nil
		}

		if err := h.svc.HandleIncoming(r.Context(), msg); err != nil {
			http.Error(w, "processing error", http.StatusInternalServerError)
			return
		}
	}

	// ACK для Chatra
	w.WriteHeader(http.StatusOK)
}
