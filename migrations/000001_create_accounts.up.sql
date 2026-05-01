CREATE TABLE IF NOT EXISTS accounts (
    id         BIGINT        PRIMARY KEY,
    name       VARCHAR(255)  NOT NULL,
    balance    NUMERIC(18,2) NOT NULL DEFAULT 0.00,
    updated_at TIMESTAMPTZ   NOT NULL DEFAULT NOW()
);
