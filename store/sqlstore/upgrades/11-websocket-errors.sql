-- v11: Add websocket errors table
CREATE TABLE IF NOT EXISTS whatsmeow_websocket_errors (
	id           SERIAL PRIMARY KEY,
	client_jid   TEXT NOT NULL,
	error_msg    TEXT NOT NULL,
	timestamp    BIGINT NOT NULL,
	processed    BOOLEAN NOT NULL DEFAULT FALSE,
	created_at   TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_whatsmeow_websocket_errors_client_jid ON whatsmeow_websocket_errors(client_jid);
CREATE INDEX IF NOT EXISTS idx_whatsmeow_websocket_errors_timestamp ON whatsmeow_websocket_errors(timestamp);
CREATE INDEX IF NOT EXISTS idx_whatsmeow_websocket_errors_processed ON whatsmeow_websocket_errors(processed);
