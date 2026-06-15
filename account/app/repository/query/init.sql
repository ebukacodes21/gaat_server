-- name: GetUsers :many
SELECT 
    id, 
    email, 
    email_verified, 
    role, 
    account_enabled, 
    last_login, 
    first_name, 
    last_name, 
    address, 
    lga, 
    zip_code, 
    state, 
    gender, 
    marital_status, 
    phone1, 
    phone2, 
    occupation, 
    img_url, 
    about_us,
    terms_accepted,
    created_at, 
    updated_at 
FROM users ORDER BY created_at DESC
LIMIT $1
OFFSET $2;

-- name: CountUsers :one
SELECT COUNT(*) AS total
FROM users;

-- name: GetUserByEmail :one
SELECT * FROM users
WHERE email = $1 LIMIT 1;

-- name: CreateUser :one
INSERT INTO users (
    email,
    password,
    first_name,
    last_name,
    address,
    lga,
    zip_code,
    state,
    gender,
    marital_status,
    phone1,
    phone2,
    occupation,
    about_us,
    verification_code,
    terms_accepted,
    verification_code_expires_at
)
VALUES (
    $1,$2,$3,$4,$5,$6,$7,$8,$9,$10,
    $11,$12,$13,$14,$15,$16,$17
)
RETURNING *;

-- name: VerifyUser :exec
UPDATE users
SET
    email_verified = true,
    account_enabled = true,
    verification_code = NULL,
    verification_code_expires_at = NOW(),
    updated_at = NOW()
WHERE id = $1;


-- name: UpdateLoanStatus :exec
UPDATE loans
SET
    status = $2,
    updated_at = NOW()
WHERE id = $1;

-- name: UpdateLoanRepayment :exec
UPDATE loans
SET
    total_repaid = $2,
    total_unpaid = $3,
    number_of_repayments = $4,
    amount_paid_towards_next_installment = $5,
    next_payment_date = $6,
    status = $7,
    updated_at = NOW()
WHERE id = $1;

-- name: UpdateDepositStatus :exec
UPDATE deposits
SET
    status = $2,
    updated_at = NOW()
WHERE id = $1;

-- name: ListLoanTypes :many
SELECT id, name, rate, created_at, is_active, updated_at 
FROM loan_types
WHERE is_active = TRUE
ORDER BY id DESC;

-- name: AdminListLoanTypes :many
SELECT * 
FROM loan_types
ORDER BY id DESC;

-- name: GetLoanTypeByID :one
SELECT id, name, rate, created_at, updated_at 
FROM loan_types
WHERE id = $1 LIMIT 1;

-- name: CreateLoanType :one
INSERT INTO loan_types (name, rate)
VALUES ($1, $2)
RETURNING id, name, rate, created_at, updated_at;

-- name: UpdateLoanType :exec
UPDATE loan_types
SET name = COALESCE(sqlc.narg(name), name),
    rate = COALESCE(sqlc.narg(rate), rate),
    is_active = COALESCE(sqlc.narg(is_active), is_active)
WHERE id = $1;

-- name: DeleteLoanType :exec
DELETE FROM loan_types
WHERE id = $1;

-- name: GetUserByID :one
SELECT
    id,
    email,
    email_verified,
    role,
    account_enabled,
    last_login,
    first_name,
    last_name,
    address,
    lga,
    zip_code,
    state,
    gender,
    marital_status,
    phone1,
    phone2,
    occupation,
    password,
    img_url,
    about_us,
    created_at,
    terms_accepted,
    updated_at
FROM users
WHERE id = $1
LIMIT 1;

-- name: UpdateLastLogin :exec
UPDATE users
SET last_login = NOW()
WHERE id = $1;

-- name: UpdatePassword :exec
UPDATE users
SET password = $2
WHERE id = $1;


-- name: UpdateStaffPassword :exec
UPDATE staffs
SET password_hash = $2
WHERE id = $1;

-- name: UpdateUserStatus :exec
UPDATE users
SET account_enabled = $2
WHERE id = $1;

-- name: UpdateUser :one
UPDATE users
SET
    first_name = COALESCE(sqlc.narg(first_name), first_name),
    last_name = COALESCE(sqlc.narg(last_name), last_name),
    address = COALESCE(sqlc.narg(address), address),
    lga = COALESCE(sqlc.narg(lga), lga),
    zip_code = COALESCE(sqlc.narg(zip_code), zip_code),
    state = COALESCE(sqlc.narg(state), state),
    gender = COALESCE(sqlc.narg(gender), gender),
    marital_status = COALESCE(sqlc.narg(marital_status), marital_status),
    phone1 = COALESCE(sqlc.narg(phone1), phone1),
    phone2 = COALESCE(sqlc.narg(phone2), phone2),
    occupation = COALESCE(sqlc.narg(occupation), occupation),
    img_url = COALESCE(sqlc.narg(img_url), img_url),
    verification_code = COALESCE(sqlc.narg(verification_code), verification_code),
    verification_code_expires_at = COALESCE(sqlc.narg(verification_code_expires_at), verification_code_expires_at),
    updated_at = NOW()
WHERE id = sqlc.arg(id)
RETURNING *;

-- name: UpdateUserRole :exec
UPDATE users
SET
    role = $2
WHERE id = $1;

-- name: DashboardStats :one
SELECT
    (SELECT COUNT(*) FROM loans) AS loans,
    (SELECT COUNT(*) FROM deposits) AS deposits,
    (SELECT COUNT(*) FROM users) AS users;

-- name: GetUserForLogin :one
SELECT
    id,
    email,
    password,
    email_verified,
    account_enabled,
    role
FROM users
WHERE email = $1
LIMIT 1;

-- name: CreateLoan :one
INSERT INTO loans (
    loan_type,
    principal_amount,
    interest_rate,
    term_months,
    monthly_payment,
    admin_fee,
    total_interest,
    total_repayment,
    total_repaid,
    total_unpaid,
    number_of_repayments,
    status,
    due_date,
    approved_date,
    next_payment_date,
    collateral,
    borrower_name,
    email,
    guarantor_name,
    guarantor_email,
    guarantor_phone,
    guarantor_ippis_no,
    bank_name,
    account_number,
    account_holder,
    bvn,
    occupation,
    employer_name,
    employer_address,
    employer_phone,
    ippis_no,
    statement,
    admin_fee_receipt,
    collateral_document,
    loan_interest,
    user_id,
    loan_type_id
)
VALUES (
    $1,$2,$3,$4,$5,$6,$7,$8,$9,$10,
    $11,$12,$13,$14,$15,$16,$17,$18,$19,$20,
    $21,$22,$23,$24,$25,$26,$27,$28,$29,$30,
    $31,$32,$33,$34,$35,$36,$37
)
RETURNING *;

-- name: CreateDeposit :one
INSERT INTO deposits (
    tx_id,
    status,
    type,
    months,
    amount,
    receipt,
    loan_id,
    user_id,
    email
)
VALUES (
    $1,$2,$3,$4,$5,$6,$7,$8,$9
)
RETURNING *;

-- name: GetLoans :many
SELECT *
FROM loans
ORDER BY created_at DESC
LIMIT $1
OFFSET $2;

-- name: GetDeposits :many
SELECT *
FROM deposits
ORDER BY created_at DESC
LIMIT $1
OFFSET $2;

-- name: GetLoansByUserID :many
SELECT *
FROM loans
WHERE user_id = $1
ORDER BY created_at DESC
LIMIT $2
OFFSET $3;

-- name: GetDepositsByUserID :many
SELECT *
FROM deposits
WHERE user_id = $1
ORDER BY created_at DESC
LIMIT $2
OFFSET $3;

-- name: GetLoanByID :one
SELECT *
FROM loans
WHERE id = $1
LIMIT 1;

-- name: GetDepositByID :one
SELECT *
FROM deposits
WHERE id = $1
LIMIT 1;

-- name: ApproveLoan :exec
UPDATE loans
SET
    status = 'approved',
    approved_date = NOW()
WHERE id = $1;

-- name: CountLoans :one
SELECT COUNT(*) AS total
FROM loans;

-- name: CountDeposits :one
SELECT COUNT(*) AS total
FROM deposits;

-- name: CountPendingLoans :one
SELECT COUNT(*) AS total
FROM loans
WHERE status = 'pending';

-- name: UpdateUserStatusAndRole :exec
UPDATE users
SET
    account_enabled = $2,
    role = $3,
    updated_at = NOW()
WHERE id = $1;

-- name: UpdateUserProfile :one
UPDATE users
SET
    first_name = COALESCE(sqlc.narg(first_name), first_name),
    last_name = COALESCE(sqlc.narg(last_name), last_name),
    address = COALESCE(sqlc.narg(address), address),
    lga = COALESCE(sqlc.narg(lga), lga),
    zip_code = COALESCE(sqlc.narg(zip_code), zip_code),
    state = COALESCE(sqlc.narg(state), state),
    gender = COALESCE(sqlc.narg(gender), gender),
    marital_status = COALESCE(sqlc.narg(marital_status), marital_status),
    phone1 = COALESCE(sqlc.narg(phone1), phone1),
    phone2 = COALESCE(sqlc.narg(phone2), phone2),
    occupation = COALESCE(sqlc.narg(occupation), occupation),
    img_url = COALESCE(sqlc.narg(img_url), img_url),
    updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: CountDepositsByUserID :one
SELECT COUNT(*)
FROM deposits
WHERE user_id = $1;

-- name: CountLoansByUserID :one
SELECT COUNT(*)
FROM loans
WHERE user_id = $1;

-- name: GetLoanTypeByName :one
SELECT * FROM loan_types
WHERE id = $1
LIMIT 1;


-- name: CreateStaff :one
INSERT INTO staffs (email, password_hash, full_name, role)
VALUES ($1, $2, $3, $4)
RETURNING id, email, full_name, role, created_at;

-- name: GetAllStaff :many
SELECT id, email, full_name, role, account_enabled, last_login_at, created_at
FROM staffs
ORDER BY created_at DESC
LIMIT $1
OFFSET $2;

-- name: GetStaffByID :one
SELECT id, email, full_name, role, account_enabled, created_at
FROM staffs
WHERE id = $1 LIMIT 1;


-- name: GetStaffWithPassword :one
SELECT id, email, full_name, password_hash, role, account_enabled, created_at
FROM staffs
WHERE id = $1 LIMIT 1;

-- name: DeleteStaff :exec
DELETE FROM staffs WHERE id = $1;

-- name: UpdateStaff :exec
UPDATE staffs
SET
    full_name = COALESCE(sqlc.narg(full_name), full_name),
    role = COALESCE(sqlc.narg(role), role),
    account_enabled = COALESCE(sqlc.narg(account_enabled), account_enabled),
    updated_at = NOW()
WHERE id = $1;

-- name: CountStaffs :one
SELECT COUNT(*) AS total
FROM staffs;


-- name: GetStaffByEmail :one
SELECT *
FROM staffs
WHERE email = $1 LIMIT 1;

-- name: UpdateLastLoginStaff :exec
UPDATE staffs
SET last_login = NOW()
WHERE id = $1;

-- name: DeleteLoans :exec
DELETE FROM loans;