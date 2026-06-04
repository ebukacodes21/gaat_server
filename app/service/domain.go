package service

import (
	"app/types"
	"context"
)

type GaatService interface {
	//users
	RegisterAccount(ctx context.Context, input *types.CreateUserInput) (*types.User, error)
	VerifyUser(ctx context.Context, email string, code string) error
	ResendOTP(ctx context.Context, email string) (*types.ServiceResult, error)
	LoginUser(ctx context.Context, email string, password string) (*types.LoginResult, error)
	Forgot(ctx context.Context, email string) (*types.ServiceResult, error)
	Reset(ctx context.Context, input types.ResetInput) (*types.ServiceResult, error)
	GetUsers(ctx context.Context, page int32, pageSize int32) (*types.UsersListResult, error)
	GetUser(ctx context.Context, id string) (*types.User, error)
	ManageUser(ctx context.Context, input types.StaffAction) error
	UpdateUser(ctx context.Context, userID string, input types.UpdateInput) (*types.User, error)
	UpdatePassword(ctx context.Context, userID string, input types.UpdatePasswordInput) (*types.ServiceResult, error)

	//loans
	ListLoanTypes(ctx context.Context) ([]*types.LoanType, error)
	AdminListLoanTypes(ctx context.Context) ([]*types.LoanType, error)
	RequestLoan(ctx context.Context, userID string, input types.RequestLoanInput) error
	GetUserLoans(ctx context.Context, userID string, page int32, pageSize int32) (*types.LoanListResult, error)
	GetUserDeposits(ctx context.Context, userID string, page int32, pageSize int32) (*types.DepositListResult, error)
	GetLoan(ctx context.Context, loanID string) (*types.Loan, error)
	GetLoans(ctx context.Context, page int32, pageSize int32) (*types.LoanListResult, error)
	ManageLoan(ctx context.Context, req types.ManageInput) error
	CreateDeposit(ctx context.Context, input types.CreateDepositInput) error
	ManageDeposit(ctx context.Context, req types.ManageInput) error
	GetDeposit(ctx context.Context, depositID string) (*types.Deposit, error)
	GetDeposits(ctx context.Context, page int32, pageSize int32) (*types.DepositListResult, error)
	CreateLoanType(ctx context.Context, req types.LoanTypeRequest) error
	UpdateLoanType(ctx context.Context, req types.UpdateLoanTypeRequest) error

	// staffs
	CreateStaff(ctx context.Context, req types.CreateStaffRequest) error
	GetStaffs(ctx context.Context, page int32, pageSize int32) (*types.StaffListResult, error)
	GetStaff(ctx context.Context, staffID string) (*types.Staff, error)
	UpdateStaff(ctx context.Context, req types.UpdateStaffRequest) error
	ManageStaff(ctx context.Context, req types.StaffAction) error
	LoginStaff(ctx context.Context, email string, password string) (*types.StaffLoginResult, error)
}
