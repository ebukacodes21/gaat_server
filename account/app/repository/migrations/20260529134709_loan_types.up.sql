-- Up Migration
CREATE TABLE loan_types (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL UNIQUE,
    rate NUMERIC(4, 2) NOT NULL DEFAULT 0.00,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- Seed Initial System Data Matrix
INSERT INTO loan_types (id, name, rate) VALUES
('a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'School Fees Loan', 0.07),
('a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a22', 'Rent Loan', 0.06),
('a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a33', 'Business Loan', 0.10),
('a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a44', 'Proof of Funds Loan', 0.04)
ON CONFLICT (id) DO UPDATE 
SET name = EXCLUDED.name, rate = EXCLUDED.rate;

-- Set up automatic updated_at timestamp management
CREATE OR REPLACE FUNCTION update_loan_types_timestamp()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trigger_update_loan_types_timestamp
BEFORE UPDATE ON loan_types
FOR EACH ROW
EXECUTE FUNCTION update_loan_types_timestamp();