-- Схема накопительной системы лояльности «Гофермарт».

CREATE TABLE IF NOT EXISTS users (
    id BIGSERIAL PRIMARY KEY,
    login TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS orders (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    number TEXT NOT NULL UNIQUE,
    status TEXT NOT NULL DEFAULT 'NEW'
        CHECK (status IN ('NEW', 'PROCESSING', 'INVALID', 'PROCESSED')),
    accrual NUMERIC(14, 2),
    uploaded_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS orders_user_id_uploaded_at_idx
    ON orders (user_id, uploaded_at);

CREATE INDEX IF NOT EXISTS orders_pending_idx
    ON orders (id) WHERE status IN ('NEW', 'PROCESSING');

CREATE TABLE IF NOT EXISTS user_balances (
    user_id BIGINT PRIMARY KEY REFERENCES users (id) ON DELETE CASCADE,
    current NUMERIC(14, 2) NOT NULL DEFAULT 0 CHECK (current >= 0),
    withdrawn NUMERIC(14, 2) NOT NULL DEFAULT 0 CHECK (withdrawn >= 0),
    version BIGINT NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS withdrawals (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    order_number TEXT NOT NULL,
    sum NUMERIC(14, 2) NOT NULL CHECK (sum > 0),
    processed_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS withdrawals_user_id_processed_at_idx
    ON withdrawals (user_id, processed_at);
