-- v11: Add websocket errors table
DO $$
BEGIN
    -- Create table only if it doesn't exist
    IF NOT EXISTS (SELECT FROM information_schema.tables WHERE table_name = 'whatsmeow_websocket_errors') THEN
        CREATE TABLE whatsmeow_websocket_errors (
            id           SERIAL PRIMARY KEY,
            client_jid   TEXT NOT NULL,
            error_msg    TEXT NOT NULL,
            timestamp    BIGINT NOT NULL,
            processed    BOOLEAN NOT NULL DEFAULT FALSE,
            created_at   TIMESTAMP DEFAULT CURRENT_TIMESTAMP
        );
    END IF;

    -- Create indexes only if they don't exist
    IF NOT EXISTS (SELECT FROM pg_indexes WHERE indexname = 'idx_whatsmeow_websocket_errors_client_jid') THEN
        CREATE INDEX idx_whatsmeow_websocket_errors_client_jid ON whatsmeow_websocket_errors(client_jid);
    END IF;

    IF NOT EXISTS (SELECT FROM pg_indexes WHERE indexname = 'idx_whatsmeow_websocket_errors_timestamp') THEN
        CREATE INDEX idx_whatsmeow_websocket_errors_timestamp ON whatsmeow_websocket_errors(timestamp);
    END IF;

    IF NOT EXISTS (SELECT FROM pg_indexes WHERE indexname = 'idx_whatsmeow_websocket_errors_processed') THEN
        CREATE INDEX idx_whatsmeow_websocket_errors_processed ON whatsmeow_websocket_errors(processed);
    END IF;
END
$$;
