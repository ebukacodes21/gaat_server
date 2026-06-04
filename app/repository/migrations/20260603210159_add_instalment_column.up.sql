ALTER TABLE loans
ADD COLUMN amount_paid_towards_next_installment NUMERIC(18,2)
NOT NULL DEFAULT 0.00;