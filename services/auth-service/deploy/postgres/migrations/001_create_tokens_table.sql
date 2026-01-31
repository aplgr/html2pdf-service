CREATE TABLE IF NOT EXISTS tokens (
    token TEXT PRIMARY KEY,
    rate_limit INTEGER NOT NULL DEFAULT 60,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    scope JSONB NOT NULL DEFAULT '{"api":true}'::jsonb,
    comment TEXT
);

CREATE INDEX IF NOT EXISTS idx_tokens_created_at ON tokens (created_at);

CREATE OR REPLACE FUNCTION fn_fetch_auth_tokens()
RETURNS TABLE (token TEXT, rate_limit INTEGER, scope JSONB)
LANGUAGE sql
STABLE
AS $$
    SELECT token, rate_limit, scope
    FROM tokens;
$$;

CREATE OR REPLACE FUNCTION fn_verify_tokens_schema() RETURNS void
LANGUAGE plpgsql
AS $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM information_schema.tables
        WHERE table_schema = 'public'
          AND table_name = 'tokens'
    ) THEN
        RAISE EXCEPTION 'auth-service schema check failed: missing tokens table';
    END IF;

    IF NOT EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_schema = 'public'
          AND table_name = 'tokens'
          AND column_name = 'token'
    ) THEN
        RAISE EXCEPTION 'auth-service schema check failed: missing tokens.token column';
    END IF;

    IF NOT EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_schema = 'public'
          AND table_name = 'tokens'
          AND column_name = 'rate_limit'
    ) THEN
        RAISE EXCEPTION 'auth-service schema check failed: missing tokens.rate_limit column';
    END IF;

    IF NOT EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_schema = 'public'
          AND table_name = 'tokens'
          AND column_name = 'created_at'
    ) THEN
        RAISE EXCEPTION 'auth-service schema check failed: missing tokens.created_at column';
    END IF;

    IF NOT EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_schema = 'public'
          AND table_name = 'tokens'
          AND column_name = 'scope'
    ) THEN
        RAISE EXCEPTION 'auth-service schema check failed: missing tokens.scope column';
    END IF;

    IF NOT EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_schema = 'public'
          AND table_name = 'tokens'
          AND column_name = 'comment'
    ) THEN
        RAISE EXCEPTION 'auth-service schema check failed: missing tokens.comment column';
    END IF;

    IF NOT EXISTS (
        SELECT 1
        FROM pg_indexes
        WHERE schemaname = 'public'
          AND tablename = 'tokens'
          AND indexname = 'idx_tokens_created_at'
    ) THEN
        RAISE EXCEPTION 'auth-service schema check failed: missing idx_tokens_created_at index';
    END IF;
END;
$$;

INSERT INTO
    tokens (token, comment)
VALUES (
        'abc123-test-first-token',
        'bootstrap token for initial testing'
    )
ON CONFLICT (token) DO NOTHING;
