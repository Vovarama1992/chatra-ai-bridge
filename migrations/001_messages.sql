CREATE TABLE messages (
  id BIGSERIAL PRIMARY KEY,
  chat_id TEXT NOT NULL,
  sender TEXT NOT NULL, -- client | supporter | ai
  text TEXT NOT NULL,
  client_id TEXT NULL,
  supporter_id TEXT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_messages_chat_id ON messages(chat_id);
CREATE INDEX idx_messages_created_at ON messages(created_at);