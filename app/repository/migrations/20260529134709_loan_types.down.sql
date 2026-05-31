-- Down Migration
DROP TRIGGER IF EXISTS trigger_update_loan_types_timestamp ON loan_types;
DROP FUNCTION IF EXISTS update_loan_types_timestamp();
DROP TABLE IF EXISTS loan_types;