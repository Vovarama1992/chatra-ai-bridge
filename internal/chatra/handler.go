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
		ChatID      string  `json:"chat_id"`
		Text        string  `json:"text"`
		ClientID    *string `json:"client_id"`
		SupporterID *string `json:"supporter_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	if payload.ChatID == "" || payload.Text == "" {
		http.Error(w, "missing chat_id or text", http.StatusBadRequest)
		return
	}

	msg := &Message{
		ChatID:      payload.ChatID,
		Sender:      SenderClient,
		Text:        payload.Text,
		ClientID:    payload.ClientID,
		SupporterID: payload.SupporterID,
	}

	reply, err := h.svc.HandleIncoming(r.Context(), msg)
	if err != nil {
		http.Error(w, "processing error", http.StatusInternalServerError)
		return
	}

	// Ответ сразу обратно в Chatra
	_ = json.NewEncoder(w).Encode(map[string]string{
		"text": reply,
	})
}
