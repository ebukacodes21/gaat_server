CREATE TABLE deposits (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tx_id UUID NOT NULL,
    status VARCHAR(100) NOT NULL,
    type VARCHAR(100) NOT NULL,
    months INTEGER NOT NULL,
    amount NUMERIC(18,2) NOT NULL DEFAULT 0.00,
    receipt TEXT NOT NULL,
    loan_id UUID NOT NULL,
    user_id UUID NOT NULL,
    email VARCHAR(255) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT fk_deposit_loan FOREIGN KEY (loan_id) REFERENCES loans(id) ON DELETE CASCADE,
    CONSTRAINT fk_deposit_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);