package types

import (
	"time"

	"github.com/shopspring/decimal"
)

type ServiceResult struct {
	Message string `json:"message"`
	Data    any    `json:"data"`
}

type EmailPayload struct {
	Email   string `json:"Email"`
	Subject string `json:"Subject"`
	Content string `json:"Content"`
}

type UpdateInput struct {
	FirstName     string `json:"first_name"`
	LastName      string `json:"last_name"`
	Address       string `json:"address"`
	Lga           string `json:"lga"`
	ZipCode       string `json:"zip_code"`
	State         string `json:"state"`
	Gender        string `json:"gender"`
	MaritalStatus string `json:"marital_status"`
	Phone1        string `json:"phone1"`
	Phone2        string `json:"phone2"`
	Occupation    string `json:"occupation"`
	ImgURL        string `json:"img_url"`
}

type LoginResult struct {
	Token          string    `json:"token"`
	ID             string    `json:"id"`
	Email          string    `json:"email"`
	FirstName      string    `json:"first_name"`
	LastName       string    `json:"last_name"`
	Role           string    `db:"role" json:"role"`
	EmailVerified  bool      `json:"email_verified"`
	AccountEnabled bool      `json:"account_enabled"`
	LastLogin      time.Time `json:"last_login,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
}

type StaffLoginResult struct {
	Token          string    `json:"token"`
	ID             string    `json:"id"`
	Email          string    `json:"email"`
	FullName       string    `json:"full_name"`
	Role           string    `db:"role" json:"role"`
	AccountEnabled bool      `json:"account_enabled"`
	LastLogin      time.Time `json:"last_login,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
}

type CreateUserInput struct {
	Email         string `json:"email"`
	Password      string `json:"password"`
	FirstName     string `json:"first_name"`
	LastName      string `json:"last_name"`
	Address       string `json:"address"`
	Lga           string `json:"lga"`
	ZipCode       string `json:"zip_code"`
	State         string `json:"state"`
	Gender        string `json:"gender"`
	MaritalStatus string `json:"marital_status"`
	AboutUs       string `json:"about_us"`
	TermsAccepted bool   `json:"terms_accepted"`
	Phone1        string `json:"phone1"`
	Phone2        string `json:"phone2"`
	Occupation    string `json:"occupation"`
}

type VerifyInput struct {
	Email string `json:"email" binding:"required,email"`
	Code  string `json:"code" binding:"required"`
}

type LoginInput struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type ForgotInput struct {
	Email string `json:"email" binding:"required,email"`
}

type ResetInput struct {
	Password string `json:"password" binding:"required"`
	Token    string `json:"token" binding:"required"`
}

type ManageInput struct {
	ID     string `json:"id" binding:"required,uuid"`
	Action string `json:"action" binding:"required,oneof=pending forwarded approved rejected repaid defaulted"`
}

type StaffAction struct {
	ID     string `json:"id" binding:"required,uuid"`
	Action string `json:"action" binding:"required,oneof=enable disable"`
}

type UpdatePasswordInput struct {
	OldPassword string `json:"old_password" binding:"required"`
	NewPassword string `json:"new_password" binding:"required"`
}

type User struct {
	ID               string    `db:"id" json:"id"`
	Email            string    `db:"email" json:"email"`
	Password         string    `db:"password" json:"-"`
	EmailVerified    bool      `db:"email_verified" json:"email_verified"`
	Role             string    `db:"role" json:"role"`
	VerificationCode string    `db:"verification_code" json:"-"`
	AccountEnabled   bool      `db:"account_enabled" json:"account_enabled"`
	LastLogin        time.Time `db:"last_login" json:"last_login"`
	FirstName        string    `db:"first_name" json:"first_name"`
	LastName         string    `db:"last_name" json:"last_name"`
	Address          string    `db:"address" json:"address"`
	Lga              string    `db:"lga" json:"lga"`
	ZipCode          string    `db:"zip_code" json:"zip_code"`
	State            string    `db:"state" json:"state"`
	Gender           string    `db:"gender" json:"gender"`
	MaritalStatus    string    `db:"marital_status" json:"marital_status"`
	Phone1           string    `db:"phone1" json:"phone1"`
	Phone2           string    `db:"phone2" json:"phone2"`
	Occupation       string    `db:"occupation" json:"occupation"`
	ImgURL           string    `db:"img_url" json:"img_url"`
	CreatedAt        time.Time `db:"created_at" json:"created_at"`
	UpdatedAt        time.Time `db:"updated_at" json:"updated_at"`
	CodeExpiresIn    time.Time `json:"code_expires_in"`
}

// loans
type LoanType struct {
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Rate      decimal.Decimal `json:"rate"`
	IsActive  bool            `json:"is_active"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
}

type LoanTypeRequest struct {
	Name string `json:"name" binding:"required"`
	Rate string `json:"rate" binding:"required"`
}

type UpdateLoanTypeRequest struct {
	ID       string `json:"id" binding:"required,uuid"`
	Name     string `json:"name" binding:"required"`
	Rate     string `json:"rate" binding:"required"`
	IsActive bool   `json:"is_active"`
}

type UpdateStaffRequest struct {
	ID       string `json:"id" binding:"required,uuid"`
	FullName string `json:"full_name" binding:"required"`
	Role     Role   `json:"role" binding:"required,oneof=admin staff supervisor"`
}

type RequestLoanInput struct {
	AccountHolder      string `json:"account_holder" binding:"required"`
	BankName           string `json:"bank_name" binding:"required"`
	AccountNumber      string `json:"account_number" binding:"required,len=10"`
	BVN                string `json:"bvn" binding:"required,len=11"`
	LoanTypeID         string `json:"loan_type_id" binding:"required,uuid"`
	TermMonths         int32  `json:"term_months" binding:"required,gt=0"`
	Collateral         string `json:"collateral" binding:"required"`
	CollateralDocument string `json:"collateral_document" binding:"required,url"`
	Occupation         string `json:"occupation" binding:"required"`
	Statement          string `json:"statement" binding:"required,url"`
	AdminFeeReceipt    string `json:"admin_fee_receipt" binding:"required,url"`
	PrincipalAmount    int64  `json:"principal_amount" binding:"required"`
	// conditionally required in service
	GuarantorName    string  `json:"guarantor_name"`
	GuarantorEmail   string  `json:"guarantor_email"`
	GuarantorPhone   string  `json:"guarantor_phone"`
	GuarantorIppisNo string  `json:"guarantor_ippis_no"`
	EmployerName     string  `json:"employer_name"`
	EmployerAddress  string  `json:"employer_address"`
	EmployerPhone    string  `json:"employer_phone"`
	IppisNo          string  `json:"ippis_no"`
	LoanInterest     string  `json:"loan_interest"`
	OverrideRate     float64 `json:"override_rate"`
}

type CreateDepositInput struct {
	LoanID  string `json:"loan_id" binding:"required,uuid"`
	Amount  string `json:"amount" binding:"required"`
	Receipt string `json:"receipt" binding:"required"`
}

type DepositActionInput struct {
	LoanID string `json:"loan_id" binding:"required,uuid"`
	TxID   string `json:"tx_id" binding:"required,uuid"`
	Email  string `json:"email" binding:"required,email"`
	Amount string `json:"amount"`
}

type CreateLoanTypeInput struct {
	Name string `json:"name" binding:"required"`
	Rate string `json:"rate" binding:"required"`
}

type UpdateLoanTypeInput struct {
	ID   string `json:"id" binding:"required,uuid"`
	Name string `json:"name" binding:"required"`
	Rate string `json:"rate" binding:"required"`
}

type Loan struct {
	ID                               string     `json:"id"`
	LoanType                         string     `json:"loan_type"`
	PrincipalAmount                  string     `json:"principal_amount"`
	InterestRate                     string     `json:"interest_rate"`
	TermMonths                       int32      `json:"term_months"`
	MonthlyPayment                   string     `json:"monthly_payment"`
	AdminFee                         string     `json:"admin_fee"`
	TotalInterest                    string     `json:"total_interest"`
	TotalRepayment                   string     `json:"total_repayment"`
	TotalRepaid                      string     `json:"total_repaid"`
	TotalUnpaid                      string     `json:"total_unpaid"`
	NumberOfRepayments               int32      `json:"number_of_repayments"`
	AmountPaidTowardsNextInstallment string     `json:"amount_paid_towards_next_installment"`
	Status                           string     `json:"status"`
	DueDate                          *time.Time `json:"due_date,omitempty"`
	ApprovedDate                     *time.Time `json:"approved_date,omitempty"`
	NextPaymentDate                  *time.Time `json:"next_payment_date,omitempty"`
	Collateral                       string     `json:"collateral"`
	BorrowerName                     string     `json:"borrower_name"`
	Email                            string     `json:"email"`
	GuarantorName                    string     `json:"guarantor_name,omitempty"`
	GuarantorEmail                   string     `json:"guarantor_email,omitempty"`
	GuarantorPhone                   string     `json:"guarantor_phone,omitempty"`
	GuarantorIppisNo                 string     `json:"guarantor_ippis_no,omitempty"`
	BankName                         string     `json:"bank_name,omitempty"`
	AccountNumber                    string     `json:"account_number,omitempty"`
	AccountHolder                    string     `json:"account_holder,omitempty"`
	BVN                              string     `json:"bvn,omitempty"`
	Occupation                       string     `json:"occupation,omitempty"`
	EmployerName                     string     `json:"employer_name,omitempty"`
	EmployerAddress                  string     `json:"employer_address,omitempty"`
	EmployerPhone                    string     `json:"employer_phone,omitempty"`
	IppisNo                          string     `json:"ippis_no,omitempty"`
	Statement                        string     `json:"statement,omitempty"`
	AdminFeeReceipt                  string     `json:"admin_fee_receipt,omitempty"`
	CollateralDocument               string     `json:"collateral_document,omitempty"`
	LoanInterest                     string     `json:"loan_interest,omitempty"`
	UserID                           string     `json:"user_id"`
	CreatedAt                        time.Time  `json:"created_at"`
	UpdatedAt                        time.Time  `json:"updated_at"`
}

type Deposit struct {
	ID        string    `json:"id"`
	TxID      string    `json:"tx_id"`
	Status    string    `json:"status"`
	Type      string    `json:"type"`
	Months    int32     `json:"months"`
	Amount    string    `json:"amount"`
	Receipt   string    `json:"receipt"`
	LoanID    string    `json:"loan_id"`
	UserID    string    `json:"user_id"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Pagination struct {
	Page       int32 `json:"page"`
	PageSize   int32 `json:"page_size"`
	Total      int64 `json:"total"`
	TotalPages int32 `json:"total_pages"`
}

type LoanListResult struct {
	Items      []*Loan    `json:"items"`
	Pagination Pagination `json:"pagination"`
}

type StaffListResult struct {
	Items      []*Staff   `json:"items"`
	Pagination Pagination `json:"pagination"`
}

type DepositListResult struct {
	Items      []*Deposit `json:"items"`
	Pagination Pagination `json:"pagination"`
}

type UsersListResult struct {
	Items      []*User    `json:"items"`
	Pagination Pagination `json:"pagination"`
}

type Role string

const (
	AdminRole      Role = "admin"
	StaffRole      Role = "staff"
	SupervisorRole Role = "supervisor"
	UserRole       Role = "user"
)

type CreateStaffRequest struct {
	Email    string `json:"email" binding:"required,email"`
	FullName string `json:"full_name" binding:"required"`
	Role     Role   `json:"role" binding:"required,oneof=admin staff supervisor"`
}

type Staff struct {
	ID             string    `json:"id" binding:"required,uuid"`
	Email          string    `json:"email" binding:"required,email"`
	FullName       string    `json:"full_name" binding:"required"`
	Role           Role      `json:"role" binding:"required,oneof=admin staff supervisor"`
	AccountEnabled bool      `json:"account_enabled"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}
