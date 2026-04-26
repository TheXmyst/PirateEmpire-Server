-- Migration: add chat_messages table (SQLite)
CREATE TABLE IF NOT EXISTS chat_messages (
    id TEXT PRIMARY KEY,
    sea_id TEXT NOT NULL,
    player_id TEXT NOT NULL,
    author TEXT NOT NULL,
    text TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_chat_messages_sea_created_at ON chat_messages(sea_id, created_at);
CREATE INDEX IF NOT EXISTS idx_chat_messages_player ON chat_messages(player_id);
