-- Схема системы расчёта вознаграждений «Гофермарт».

CREATE TABLE IF NOT EXISTS goods (
    id BIGSERIAL PRIMARY KEY,
    match TEXT NOT NULL UNIQUE CHECK (match <> ''),
    reward NUMERIC(14, 2) NOT NULL CHECK (reward >= 0),
    reward_type TEXT NOT NULL CHECK (reward_type IN ('%', 'pt'))
);

CREATE TABLE IF NOT EXISTS accrual_orders (
    id BIGSERIAL PRIMARY KEY,
    number TEXT NOT NULL UNIQUE,
    status TEXT NOT NULL DEFAULT 'REGISTERED'
        CHECK (status IN ('REGISTERED', 'INVALID', 'PROCESSING', 'PROCESSED')),
    accrual NUMERIC(14, 2),
    uploaded_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    processed_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS accrual_orders_pending_idx
    ON accrual_orders (id) WHERE status IN ('REGISTERED', 'PROCESSING');

CREATE TABLE IF NOT EXISTS accrual_order_items (
    id BIGSERIAL PRIMARY KEY,
    order_id BIGINT NOT NULL REFERENCES accrual_orders (id) ON DELETE CASCADE,
    description TEXT NOT NULL,
    price NUMERIC(14, 2) NOT NULL CHECK (price >= 0)
);

CREATE INDEX IF NOT EXISTS accrual_order_items_order_id_idx
    ON accrual_order_items (order_id);
