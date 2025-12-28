CREATE TABLE IF NOT EXISTS tokens (
    token TEXT PRIMARY KEY,
    rate_limit INTEGER NOT NULL DEFAULT 60,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    comment TEXT
);

CREATE INDEX IF NOT EXISTS idx_tokens_created_at ON tokens (created_at);

INSERT INTO
    tokens (token, comment)
VALUES (
        'abc123-test-first-token',
        'bootstrap token for initial testing'
    )
ON CONFLICT (token) DO NOTHING;