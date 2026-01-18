package chatra

import (
	"context"
	"database/sql"
)

type repo struct {
	db *sql.DB
}

func NewRepo(db *sql.DB) Repo {
	return &repo{db: db}
}

func (r *repo) SaveMessage(ctx context.Context, msg *Message) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO messages (chat_id, sender, text, client_id, supporter_id)
		VALUES ($1, $2, $3, $4, $5)
	`,
		msg.ChatID,
		string(msg.Sender),
		msg.Text,
		msg.ClientID,
		msg.SupporterID,
	)
	return err
}

func (r *repo) GetHistory(ctx context.Context, chatID string) ([]Message, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, chat_id, sender, text, client_id, supporter_id, extract(epoch from created_at)::bigint
		FROM messages
		WHERE chat_id = $1
		ORDER BY created_at ASC
	`, chatID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Message
	for rows.Next() {
		var m Message
		var sender string
		if err := rows.Scan(
			&m.ID,
			&m.ChatID,
			&sender,
			&m.Text,
			&m.ClientID,
			&m.SupporterID,
			&m.CreatedAt,
		); err != nil {
			return nil, err
		}
		m.Sender = Sender(sender)
		out = append(out, m)
	}

	return out, rows.Err()
}
