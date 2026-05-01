CREATE TABLE IF NOT EXISTS transactions (
    id             VARCHAR(26)   PRIMARY KEY,
    source_account BIGINT        NOT NULL REFERENCES accounts(id),
    dest_account   BIGINT        NOT NULL REFERENCES accounts(id),
    amount         NUMERIC(18,2) NOT NULL,
    status         VARCHAR(20)   NOT NULL DEFAULT 'pending',
    created_at     TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ   NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_transactions_source
    ON transactions(source_account);
CREATE INDEX IF NOT EXISTS idx_transactions_dest
    ON transactions(dest_account);
CREATE INDEX IF NOT EXISTS idx_transactions_status_pending
    ON transactions(status) WHERE status = 'pending';
