package service

import (
	repository "app/repository/sqlc"
	"app/types"
	"app/utils"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"math"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"
)

type Service struct {
	logger    *zap.Logger
	snsClient *sns.Client
	s3Bucket  *s3.Client
	repo      repository.DatabaseContract
	maker     utils.TokenMaker
	mailer    utils.Mailer
}

func NewService(logger *zap.Logger, repo repository.DatabaseContract, maker utils.TokenMaker, mailer utils.Mailer) GaatService {
	cfg, err := config.LoadDefaultConfig(context.Background(), config.WithRegion(os.Getenv("APP_AWS_REGION")))
	if err != nil {
		logger.Error("unable to load AWS configuration", zap.Any("err", err))
		panic(err)
	}

	snsClient := sns.NewFromConfig(cfg)
	s3Bucket := s3.NewFromConfig(cfg)

	return &Service{
		logger:    logger,
		snsClient: snsClient,
		s3Bucket:  s3Bucket,
		repo:      repo,
		maker:     maker,
		mailer:    mailer,
	}
}

func (s *Service) RegisterAccount(ctx context.Context, input *types.CreateUserInput) (*types.User, error) {
	hashedPassword, err := utils.HashPassword(input.Password)
	if err != nil {
		return nil, utils.NewInternalError("unable to hash password", err)
	}

	expiry := time.Now().Add(time.Minute * 3)
	code := utils.GenerateCode()
	args := repository.CreateAccountTxParams{
		CreateUserParams: repository.CreateUserParams{
			Email:                     input.Email,
			Password:                  hashedPassword,
			FirstName:                 input.FirstName,
			LastName:                  input.LastName,
			Phone1:                    input.Phone1,
			Phone2:                    input.Phone2,
			Address:                   input.Address,
			Lga:                       input.Lga,
			State:                     input.State,
			ZipCode:                   input.ZipCode,
			Gender:                    input.Gender,
			MaritalStatus:             input.MaritalStatus,
			Occupation:                input.Occupation,
			AboutUs:                   input.AboutUs,
			VerificationCode:          sql.NullString{String: code, Valid: true},
			VerificationCodeExpiresAt: sql.NullTime{Time: expiry, Valid: true},
		},
		AfterCreate: func(u repository.User) error {
			title := "Verification Code"
			body := fmt.Sprintf(`
				<p>Hello %s,</p>
				<p>Welcome to GAAT Investment. Please use the verification code below to proceed:</p>
				<div style="background: #211E1B; padding: 15px; text-align: center; font-size: 24px; font-weight: bold; color: #E6A15C; letter-spacing: 5px; border-radius: 8px; margin: 20px 0;">
					%s
				</div>
				<p style="font-size: 12px; color: #A39990;">This code expires on %s.</p>
			`, u.FirstName, u.VerificationCode.String, expiry.Format("02 Jan 2006 15:04 PM"))
			action := ""
			content := fmt.Sprintf(utils.EmailTemplate, title, body, action)

			if err := s.mailer.SendMail("Welcome Mail", content, []string{u.Email}, nil, nil, nil); err != nil {
				s.logger.Error("failed to send email %v", zap.Error(err))
			}
			return nil
		},
	}

	user, err := s.repo.CreateUserAccountTx(ctx, args)
	if err != nil {
		log.Print(err)
		return nil, err
	}

	return &types.User{
		ID:             user.ID.String(),
		Email:          user.Email,
		FirstName:      user.FirstName,
		LastName:       user.LastName,
		EmailVerified:  user.EmailVerified,
		AccountEnabled: user.AccountEnabled,
		Role:           string(user.Role),
		CreatedAt:      user.CreatedAt,
		UpdatedAt:      user.UpdatedAt,
		CodeExpiresIn:  user.VerificationCodeExpiresAt.Time,
	}, nil
}

func (s *Service) VerifyUser(ctx context.Context, email string, code string) error {
	user, err := s.repo.GetUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return utils.NewNotFoundError(fmt.Sprintf("account not found with email: %s", email), err)
		}
		return utils.NewInternalError(fmt.Sprintf("unable to find account with email: %s", email), err)
	}

	if user.EmailVerified {
		return utils.NewAlreadyExistsError(fmt.Sprintf("email: %s already verified", email), err)
	}

	if time.Now().After(user.VerificationCodeExpiresAt.Time) {
		return utils.NewPermissionDeniedError(fmt.Sprintf("code :%s expired. request a new one", code))
	}

	if user.VerificationCode.String != code {
		return utils.NewUnathenticatedError(fmt.Sprintf("invalid verification code: %s", code))
	}

	err = s.repo.VerifyUser(ctx, user.ID)
	if err != nil {
		return utils.NewInternalError("failed to verify user", err)
	}

	return nil
}

func (s *Service) ResendOTP(ctx context.Context, email string) (*types.ServiceResult, error) {
	user, err := s.repo.GetUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, utils.NewNotFoundError(fmt.Sprintf("account not found with email: %s", email), err)
		}
		return nil, utils.NewInternalError(fmt.Sprintf("unable to find account with email: %s", email), err)
	}

	if user.EmailVerified {
		return nil, utils.NewAlreadyExistsError(fmt.Sprintf("email: %s already verified", email), err)
	}

	if time.Now().Before(user.VerificationCodeExpiresAt.Time) {
		return nil, utils.NewPermissionDeniedError("code is yet to expire")
	}

	code := utils.GenerateCode()
	expiry := time.Now().Add(time.Minute * 3)

	title := "Resend Verification Code"
	body := fmt.Sprintf(`
		<p>Hello %s,</p>
		<p>You recently requested to resend your verification code for GAAT Investment.</p>
		<p>Your new verification code is:</p>
		<div style="background: #211E1B; padding: 15px; text-align: center; font-size: 24px; font-weight: bold; color: #E6A15C; letter-spacing: 5px; border-radius: 8px; margin: 20px 0;">
			%s
		</div>
		<p style="font-size: 12px; color: #A39990;">This code expires on %s.</p>
	`, user.FirstName, code, expiry.Format("02 Jan 2006 15:04 PM"))

	action := ""

	// 3. Assemble using your base email template
	content := fmt.Sprintf(utils.EmailTemplate, title, body, action)

	_, err = s.repo.UpdateUser(ctx, repository.UpdateUserParams{
		VerificationCode:          sql.NullString{String: code, Valid: true},
		VerificationCodeExpiresAt: sql.NullTime{Time: expiry, Valid: true},
		ID:                        user.ID,
	})

	if err != nil {
		return nil, utils.NewInternalError("unable to update verification params", err)
	}

	if err := s.mailer.SendMail("Resend Mail", content, []string{user.Email}, nil, nil, nil); err != nil {
		s.logger.Error("error sending email: %v", zap.Error(err))
	}

	return &types.ServiceResult{
		Data: map[string]any{
			"code_expires_at": expiry,
		},
	}, nil
}

func (s *Service) LoginUser(ctx context.Context, email string, password string) (*types.LoginResult, error) {
	user, err := s.repo.GetUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, utils.NewNotFoundError(fmt.Sprintf("account not found with email: %s", email), err)
		}
		return nil, utils.NewInternalError(fmt.Sprintf("unable to find account with email: %s", email), err)
	}

	if !user.EmailVerified {
		return nil, utils.NewUnathenticatedError("please verify email address to login")
	}

	if !user.AccountEnabled {
		return nil, utils.NewPermissionDeniedError("your account is temporarily disabled")
	}

	err = utils.CheckPassword(password, user.Password)
	if err != nil {
		return nil, utils.NewUnathenticatedError("incorrect password")
	}

	err = s.repo.UpdateLastLogin(ctx, user.ID)
	if err != nil {
		s.logger.Warn("failed updating last login", zap.Error(err))
	}

	token, _, err := s.maker.CreateToken(user.Email, user.ID.String(), string(user.Role), user.EmailVerified, 12*time.Hour)
	if err != nil {
		return nil, utils.NewInternalError("unable to create access token", err)
	}

	return &types.LoginResult{
		Token:          token,
		ID:             user.ID.String(),
		Email:          user.Email,
		FirstName:      user.FirstName,
		LastName:       user.LastName,
		Role:           string(user.Role),
		EmailVerified:  user.EmailVerified,
		AccountEnabled: user.AccountEnabled,
		LastLogin:      user.LastLogin,
		CreatedAt:      user.CreatedAt,
	}, nil
}

func (s *Service) Forgot(ctx context.Context, email string) (*types.ServiceResult, error) {
	user, err := s.repo.GetUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return &types.ServiceResult{
				Message: "a reset link has been sent to the email address, follow the link to reset your password",
			}, nil
		}

		return &types.ServiceResult{}, utils.NewInternalError("unable to process password reset request", err)
	}

	token, _, err := s.maker.CreateToken(user.Email, user.ID.String(), string(user.Role), user.EmailVerified, 5*time.Minute)
	if err != nil {
		return nil, utils.NewInternalError("failed to generate reset token", err)
	}

	resetURL := fmt.Sprintf("%s/auth/reset?token=%s", os.Getenv("FRONTEND_URL"), token)

	title := "Password Reset Request"

	body := fmt.Sprintf(`
    <p>Hello %s %s,</p>
    <p>You requested to reset your GAAT Investment password. If you did not make this request, please ignore this email.</p>
    <p style="color: #A39990; font-size: 14px;">This link will expire in 5 minutes.</p>
	`, user.FirstName, user.LastName)

	action := fmt.Sprintf(`
    <div style="text-align: center; margin: 25px 0;">
        <a href="%s" style="background-color: #E6A15C; color: #1A1816; padding: 12px 24px; text-decoration: none; border-radius: 8px; font-weight: bold; display: inline-block;">
            Reset My Password
        </a>
    </div>
	`, resetURL)

	content := fmt.Sprintf(utils.EmailTemplate, title, body, action)

	if err := s.mailer.SendMail("Password Reset Request", content, []string{user.Email}, nil, nil, nil); err != nil {
		s.logger.Error("failed sending password reset email", zap.Error(err))
	}

	return &types.ServiceResult{
		Message: "a reset link has been sent to the email address, follow the link to reset your password",
	}, nil
}

func (s *Service) Reset(ctx context.Context, input types.ResetInput) (*types.ServiceResult, error) {
	payload, err := s.maker.VerifyToken(input.Token)
	if err != nil {
		return nil, utils.NewUnathenticatedError("invalid or expired token, please request a fresh reset link")
	}

	user, err := s.repo.GetUserByEmail(ctx, payload.Email)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, utils.NewNotFoundError("no user found", err)
		}

		return nil, utils.NewInternalError("unable to fetch user", err)
	}

	hashedPassword, err := utils.HashPassword(input.Password)
	if err != nil {
		return nil, utils.NewInternalError("unable to hash password", err)
	}

	err = s.repo.UpdatePassword(ctx, repository.UpdatePasswordParams{
		ID:       user.ID,
		Password: hashedPassword,
	})
	if err != nil {
		return nil, utils.NewInternalError("failed to update password", err)
	}

	return &types.ServiceResult{
		Message: "password reset successful",
	}, nil
}

func (s *Service) GetUsers(ctx context.Context, page int32, pageSize int32) (*types.UsersListResult, error) {
	if page < 1 {
		page = 1
	}

	if pageSize < 1 {
		pageSize = 10
	}

	offset := (page - 1) * pageSize

	total, err := s.repo.CountUsers(ctx)
	if err != nil {
		return nil, utils.NewInternalError("unable to count user loans", err)
	}

	rows, err := s.repo.GetUsers(ctx, repository.GetUsersParams{
		Limit:  pageSize,
		Offset: offset,
	})
	if err != nil {
		return nil, utils.NewInternalError("unable to fetch users", err)
	}

	users := make([]*types.User, len(rows))

	for i, user := range rows {
		users[i] = &types.User{
			ID:             user.ID.String(),
			Email:          user.Email,
			FirstName:      user.FirstName,
			LastName:       user.LastName,
			EmailVerified:  user.EmailVerified,
			AccountEnabled: user.AccountEnabled,
			Role:           string(user.Role),
			Address:        user.Address,
			Lga:            user.Lga,
			State:          user.State,
			ZipCode:        user.ZipCode,
			Gender:         user.Gender,
			MaritalStatus:  user.MaritalStatus,
			Phone1:         user.Phone1,
			Phone2:         user.Phone2,
			Occupation:     user.Occupation,
			ImgURL:         user.ImgUrl,
			LastLogin:      user.LastLogin,
			CreatedAt:      user.CreatedAt,
			UpdatedAt:      user.UpdatedAt,
		}
	}

	return &types.UsersListResult{
		Items: users,
		Pagination: types.Pagination{
			Page:       page,
			PageSize:   pageSize,
			Total:      total,
			TotalPages: int32(math.Ceil(float64(total) / float64(pageSize))),
		},
	}, nil
}

func (s *Service) GetUser(ctx context.Context, userID string) (*types.User, error) {
	id, err := utils.ParseUUID(userID)
	if err != nil {
		return nil, err
	}

	user, err := s.repo.GetUserByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, utils.NewNotFoundError("user not found", err)
		}

		return nil, utils.NewInternalError("failed to fetch user", err)
	}

	return &types.User{
		ID:             user.ID.String(),
		Email:          user.Email,
		EmailVerified:  user.EmailVerified,
		Role:           string(user.Role),
		AccountEnabled: user.AccountEnabled,
		LastLogin:      user.LastLogin,
		FirstName:      user.FirstName,
		LastName:       user.LastName,
		Address:        user.Address,
		Lga:            user.Lga,
		ZipCode:        user.ZipCode,
		State:          user.State,
		Gender:         user.Gender,
		MaritalStatus:  user.MaritalStatus,
		Phone1:         user.Phone1,
		Phone2:         user.Phone2,
		Occupation:     user.Occupation,
		ImgURL:         user.ImgUrl,
		CreatedAt:      user.CreatedAt,
		UpdatedAt:      user.UpdatedAt,
	}, nil
}

func (s *Service) ManageUser(ctx context.Context, input types.StaffAction) error {
	id, err := utils.ParseUUID(input.ID)
	if err != nil {
		return err
	}

	user, err := s.repo.GetUserByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return utils.NewNotFoundError("user not found", err)
		}

		return utils.NewInternalError("failed to fetch user", err)
	}

	var ok bool
	switch input.Action {
	case "disable":
		ok = false
	case "enable":
		ok = true

	default:
		return utils.NewInvalidArgumentError("invalid action")
	}

	err = s.repo.UpdateUserStatus(ctx, repository.UpdateUserStatusParams{
		ID:             user.ID,
		AccountEnabled: ok,
	})
	if err != nil {
		return utils.NewInternalError("failed to update user", err)
	}

	return nil
}

func (s *Service) UpdateUser(ctx context.Context, userID string, input types.UpdateInput) (*types.User, error) {
	id, err := utils.ParseUUID(userID)
	if err != nil {
		return nil, err
	}

	updated, err := s.repo.UpdateUserProfile(ctx, repository.UpdateUserProfileParams{
		ID:            id,
		FirstName:     sql.NullString{String: input.FirstName, Valid: input.FirstName != ""},
		LastName:      sql.NullString{String: input.LastName, Valid: input.LastName != ""},
		Address:       sql.NullString{String: input.Address, Valid: input.Address != ""},
		Lga:           sql.NullString{String: input.Lga, Valid: input.Lga != ""},
		ZipCode:       sql.NullString{String: input.ZipCode, Valid: input.ZipCode != ""},
		State:         sql.NullString{String: input.State, Valid: input.State != ""},
		Gender:        sql.NullString{String: input.Gender, Valid: input.Gender != ""},
		MaritalStatus: sql.NullString{String: input.MaritalStatus, Valid: input.MaritalStatus != ""},
		Phone1:        sql.NullString{String: input.Phone1, Valid: input.Phone1 != ""},
		Phone2:        sql.NullString{String: input.Phone2, Valid: input.Phone2 != ""},
		Occupation:    sql.NullString{String: input.Occupation, Valid: input.Occupation != ""},
		ImgUrl:        sql.NullString{String: input.ImgURL, Valid: input.ImgURL != ""},
	},
	)
	if err != nil {
		return nil, utils.NewInternalError("failed to update user", err)
	}

	return &types.User{
		ID:             updated.ID.String(),
		Email:          updated.Email,
		Role:           string(updated.Role),
		EmailVerified:  updated.EmailVerified,
		AccountEnabled: updated.AccountEnabled,
		FirstName:      updated.FirstName,
		LastName:       updated.LastName,
		Address:        updated.Address,
		Lga:            updated.Lga,
		ZipCode:        updated.ZipCode,
		State:          updated.State,
		Gender:         updated.Gender,
		MaritalStatus:  updated.MaritalStatus,
		Phone1:         updated.Phone1,
		Phone2:         updated.Phone2,
		Occupation:     updated.Occupation,
		ImgURL:         updated.ImgUrl,
		CreatedAt:      updated.CreatedAt,
		UpdatedAt:      updated.UpdatedAt,
	}, nil
}

func (s *Service) UpdatePassword(ctx context.Context, userID string, input types.UpdatePasswordInput) (*types.ServiceResult, error) {
	id, err := utils.ParseUUID(userID)
	if err != nil {
		return nil, err
	}

	user, err := s.repo.GetUserByID(ctx, id)
	if err == nil {
		if err := utils.CheckPassword(input.OldPassword, user.Password); err != nil {
			return nil, utils.NewUnathenticatedError("incorrect current password")
		}

		hashedPassword, err := utils.HashPassword(input.NewPassword)
		if err != nil {
			return nil, utils.NewInternalError("failed to hash password", err)
		}

		if err := s.repo.UpdatePassword(ctx, repository.UpdatePasswordParams{
			ID:       user.ID,
			Password: hashedPassword,
		}); err != nil {
			return nil, utils.NewInternalError("failed to update password", err)
		}

		return &types.ServiceResult{Message: "password updated successfully"}, nil
	}

	if !errors.Is(err, sql.ErrNoRows) {
		return nil, utils.NewInternalError("failed to fetch user", err)
	}

	staff, err := s.repo.GetStaffWithPassword(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, utils.NewNotFoundError("user or staff not found", err)
		}
		return nil, utils.NewInternalError("failed to fetch staff", err)
	}

	if err := utils.CheckPassword(input.OldPassword, staff.PasswordHash); err != nil {
		return nil, utils.NewUnathenticatedError("incorrect current password")
	}

	hashedPassword, err := utils.HashPassword(input.NewPassword)
	if err != nil {
		return nil, utils.NewInternalError("failed to hash password", err)
	}

	if err := s.repo.UpdateStaffPassword(ctx, repository.UpdateStaffPasswordParams{
		ID:           staff.ID,
		PasswordHash: hashedPassword,
	}); err != nil {
		return nil, utils.NewInternalError("failed to update staff password", err)
	}

	return &types.ServiceResult{Message: "password updated successfully"}, nil
}

func (s *Service) ListLoanTypes(ctx context.Context) ([]*types.LoanType, error) {
	rows, err := s.repo.ListLoanTypes(ctx)
	if err != nil {
		log.Print("unable to list loan types", err)
		return nil, utils.NewInternalError("unable to list loan types", err)
	}

	lt := make([]*types.LoanType, len(rows))
	for i, v := range rows {
		r, err := decimal.NewFromString(v.Rate)
		if err != nil {
			return nil, utils.NewInternalError("invalid rate", err)
		}

		lt[i] = &types.LoanType{
			ID:        v.ID.String(),
			Name:      v.Name,
			Rate:      r,
			CreatedAt: v.CreatedAt,
		}
	}
	return lt, nil
}

func (s *Service) GetUserLoans(ctx context.Context, userID string, page int32, pageSize int32) (*types.LoanListResult, error) {
	id, err := utils.ParseUUID(userID)
	if err != nil {
		return nil, err
	}

	if page < 1 {
		page = 1
	}

	if pageSize < 1 {
		pageSize = 10
	}

	offset := (page - 1) * pageSize

	total, err := s.repo.CountLoansByUserID(ctx, id)
	if err != nil {
		return nil, utils.NewInternalError("unable to count user loans", err)
	}

	list, err := s.repo.GetLoansByUserID(ctx, repository.GetLoansByUserIDParams{
		UserID: id,
		Limit:  pageSize,
		Offset: offset,
	})
	if err != nil {
		return nil, utils.NewInternalError("unable to get user loans", err)
	}

	loans := make([]*types.Loan, len(list))
	for i, l := range list {
		loans[i] = mapLoan(l)
	}

	return &types.LoanListResult{
		Items: loans,
		Pagination: types.Pagination{
			Page:       page,
			PageSize:   pageSize,
			Total:      total,
			TotalPages: int32(math.Ceil(float64(total) / float64(pageSize))),
		},
	}, nil
}

func (s *Service) GetUserDeposits(ctx context.Context, userID string, page int32, pageSize int32) (*types.DepositListResult, error) {
	id, err := utils.ParseUUID(userID)
	if err != nil {
		return nil, err
	}

	if page < 1 {
		page = 1
	}

	if pageSize < 1 {
		pageSize = 10
	}

	offset := (page - 1) * pageSize

	total, err := s.repo.CountDepositsByUserID(ctx, id)
	if err != nil {
		return nil, utils.NewInternalError("unable to count user deposits", err)
	}

	list, err := s.repo.GetDepositsByUserID(ctx, repository.GetDepositsByUserIDParams{
		UserID: id,
		Limit:  pageSize,
		Offset: offset,
	})
	if err != nil {
		return nil, utils.NewInternalError("unable to get user deposits", err)
	}

	deposits := make([]*types.Deposit, len(list))
	for i, d := range list {
		deposits[i] = mapDeposit(d)
	}

	totalPages := int32(math.Ceil(float64(total) / float64(pageSize)))

	return &types.DepositListResult{
		Items: deposits,
		Pagination: types.Pagination{
			Page:       page,
			PageSize:   pageSize,
			Total:      total,
			TotalPages: totalPages,
		},
	}, nil
}

func (s *Service) RequestLoan(ctx context.Context, userID string, input types.RequestLoanInput) error {
	id, err := utils.ParseUUID(userID)
	if err != nil {
		return err
	}

	ltid, err := utils.ParseUUID(input.LoanTypeID)
	if err != nil {
		return err
	}

	user, err := s.repo.GetUserByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return utils.NewNotFoundError("user not found", err)
		}

		return utils.NewInternalError("failed to fetch user", err)
	}

	loanType, err := s.repo.GetLoanTypeByName(ctx, ltid)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return utils.NewNotFoundError("loan type not found", err)
		}

		return utils.NewInternalError("failed to fetch loan type", err)
	}

	isPofLoan := loanType.Name == "Proof of Funds Loan"

	occupation := strings.TrimSpace(input.Occupation)
	if !isPofLoan {
		switch occupation {
		case "Employed":
			if strings.TrimSpace(input.EmployerAddress) == "" ||
				strings.TrimSpace(input.EmployerName) == "" ||
				strings.TrimSpace(input.EmployerPhone) == "" {

				return utils.NewInvalidArgumentError(
					"all employer details must be provided for employed users",
				)
			}

			if strings.TrimSpace(input.GuarantorEmail) == "" ||
				strings.TrimSpace(input.GuarantorIppisNo) == "" ||
				strings.TrimSpace(input.GuarantorName) == "" ||
				strings.TrimSpace(input.GuarantorPhone) == "" {

				return utils.NewInvalidArgumentError(
					"all guarantor details must be provided for employed users",
				)
			}

			if err := utils.ParseEmail(input.GuarantorEmail); err != nil {
				return err
			}

			if err := utils.ParsePhone(input.GuarantorPhone); err != nil {
				return err
			}

			if err := utils.ParsePhone(input.EmployerPhone); err != nil {
				return err
			}

		case "Self-Employed", "Unemployed", "Student":
			if strings.TrimSpace(input.GuarantorEmail) == "" ||
				strings.TrimSpace(input.GuarantorIppisNo) == "" ||
				strings.TrimSpace(input.GuarantorName) == "" ||
				strings.TrimSpace(input.GuarantorPhone) == "" {

				return utils.NewInvalidArgumentError(
					"all guarantor details must be provided for your occupation",
				)
			}

			if err := utils.ParseEmail(input.GuarantorEmail); err != nil {
				return err
			}

			if err := utils.ParsePhone(input.GuarantorPhone); err != nil {
				return err
			}

		default:
			return utils.NewInvalidArgumentError(
				"occupation must be one of: Employed, Self-Employed, Unemployed, Student",
			)
		}
	}

	if isPofLoan {
		if strings.TrimSpace(input.LoanInterest) == "" {
			return utils.NewPermissionDeniedError(
				"proof of funds interest payment evidence is required",
			)
		}

		if err := utils.ParseURL(input.LoanInterest); err != nil {
			return err
		}
	}

	amount := float64(input.PrincipalAmount)

	rate := 0.0

	if strings.TrimSpace(input.OverrideRate) != "" {
		rate, err = strconv.ParseFloat(input.OverrideRate, 64)
		if err != nil {
			return utils.NewInvalidArgumentError("invalid override rate")
		}

		rate = rate / 100
	} else {
		rate, err = strconv.ParseFloat(loanType.Rate, 64)
		if err != nil {
			return utils.NewInternalError("invalid interest rate", err)
		}
	}

	calc := utils.CalculateLoan(amount, int(input.TermMonths), rate)

	monthlyPayment := calc.MonthlyPayment
	totalInterest := calc.TotalInterest
	totalRepayment := calc.TotalRepayment

	adminFee := 0.01 * amount
	if isPofLoan {
		adminFee = 0.005 * amount
	}

	dueDate := time.Now().AddDate(0, int(input.TermMonths), 0)

	payload := repository.CreateLoanParams{
		LoanType:           loanType.Name,
		PrincipalAmount:    fmt.Sprintf("%.2f", amount),
		InterestRate:       fmt.Sprintf("%.2f", rate),
		Email:              user.Email,
		BorrowerName:       fmt.Sprintf("%s %s", user.LastName, user.FirstName),
		MonthlyPayment:     sql.NullString{Valid: monthlyPayment > 0, String: fmt.Sprintf("%.2f", monthlyPayment)},
		TotalInterest:      sql.NullString{Valid: true, String: fmt.Sprintf("%.2f", totalInterest)},
		AdminFee:           fmt.Sprintf("%.2f", adminFee),
		TotalRepayment:     sql.NullString{Valid: true, String: fmt.Sprintf("%.2f", totalRepayment)},
		DueDate:            sql.NullTime{Valid: !dueDate.IsZero(), Time: dueDate},
		Status:             "pending",
		Collateral:         input.Collateral,
		CollateralDocument: sql.NullString{Valid: input.CollateralDocument != "", String: input.CollateralDocument},
		AccountHolder:      sql.NullString{Valid: input.AccountHolder != "", String: input.AccountHolder},
		BankName:           sql.NullString{Valid: input.BankName != "", String: input.BankName},
		AccountNumber:      sql.NullString{Valid: input.AccountNumber != "", String: input.AccountNumber},
		Bvn:                sql.NullString{Valid: input.BVN != "", String: input.BVN},
		Occupation:         sql.NullString{Valid: input.Occupation != "", String: input.Occupation},
		EmployerName:       sql.NullString{Valid: input.EmployerName != "", String: input.EmployerName},
		EmployerPhone:      sql.NullString{Valid: input.EmployerPhone != "", String: input.EmployerPhone},
		EmployerAddress:    sql.NullString{Valid: input.EmployerAddress != "", String: input.EmployerAddress},
		GuarantorName:      sql.NullString{Valid: input.GuarantorName != "", String: input.GuarantorName},
		GuarantorEmail:     sql.NullString{Valid: input.GuarantorEmail != "", String: input.GuarantorEmail},
		GuarantorPhone:     sql.NullString{Valid: input.GuarantorPhone != "", String: input.GuarantorPhone},
		GuarantorIppisNo:   sql.NullString{Valid: input.GuarantorIppisNo != "", String: input.GuarantorIppisNo},
		Statement:          sql.NullString{Valid: input.Statement != "", String: input.Statement},
		AdminFeeReceipt:    sql.NullString{Valid: input.AdminFeeReceipt != "", String: input.AdminFeeReceipt},
		LoanInterest:       sql.NullString{Valid: input.LoanInterest != "", String: input.LoanInterest},
		TermMonths:         sql.NullInt32{Valid: input.TermMonths > 0, Int32: input.TermMonths},
		UserID:             user.ID,
		TotalRepaid:        "0.00",
		TotalUnpaid:        fmt.Sprintf("%.2f", totalRepayment),
		LoanTypeID:         ltid,
	}

	_, err = s.repo.CreateLoan(ctx, payload)
	if err != nil {
		log.Println(err)
		return utils.NewInternalError("unable to initiate loan request", err)
	}

	return nil
}

func (s *Service) ManageLoan(ctx context.Context, req types.ManageInput) error {
	id, err := utils.ParseUUID(req.ID)
	if err != nil {
		return err
	}

	loan, err := s.repo.GetLoanByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return utils.NewNotFoundError("loan not found", err)
		}
		return utils.NewInternalError("failed to fetch loan", err)
	}

	action := strings.ToLower(strings.TrimSpace(req.Action))

	valid := map[string]struct{}{
		"pending":   {},
		"forwarded": {},
		"approved":  {},
		"rejected":  {},
		"repaid":    {},
		"defaulted": {},
	}

	if _, ok := valid[action]; !ok {
		return utils.NewInvalidArgumentError(
			"action must be one of: pending, approved, active, repaid, defaulted, rejected",
		)
	}

	curent := strings.ToLower(strings.TrimSpace(loan.Status))

	// idempotency
	if curent == action {
		return nil // no-op
	}

	terminal := map[string]bool{
		"rejected":  true,
		"repaid":    true,
		"defaulted": true,
	}

	if terminal[curent] {
		return utils.NewInvalidArgumentError(
			"loan is already in a terminal state",
		)
	}

	// rules
	allowed := map[string][]string{
		"pending":   {"forwarded"},
		"forwarded": {"approved", "rejected"},
		"approved":  {"repaid", "defaulted"},
		"rejected":  {},
		"repaid":    {},
		"defaulted": {},
	}

	current := strings.ToLower(loan.Status)
	ok := false

	for _, v := range allowed[current] {
		if v == action {
			ok = true
			break
		}
	}

	if !ok {
		return utils.NewInvalidArgumentError(
			fmt.Sprintf("cannot transition loan from %s to %s", current, action),
		)
	}

	err = s.repo.UpdateLoanStatus(ctx, repository.UpdateLoanStatusParams{
		Status: action,
		ID:     id,
	})
	if err != nil {
		return utils.NewInternalError("failed to update loan status", err)
	}

	link := fmt.Sprintf("%s/dashboard", os.Getenv("FRONTEND_URL"))
	if err := s.sendLoanEmail(loan, action, link); err != nil {
		s.logger.Error("unable to send loan application mail", zap.Error(err))
	}

	return nil
}

func (s *Service) GetLoans(ctx context.Context, page int32, pageSize int32) (*types.LoanListResult, error) {
	if page < 1 {
		page = 1
	}

	if pageSize < 1 {
		pageSize = 10
	}

	offset := (page - 1) * pageSize

	total, err := s.repo.CountLoans(ctx)
	if err != nil {
		return nil, utils.NewInternalError("unable to count user loans", err)
	}

	list, err := s.repo.GetLoans(ctx, repository.GetLoansParams{
		Limit:  pageSize,
		Offset: offset,
	})
	if err != nil {
		return nil, utils.NewInternalError("unable to get loans", err)
	}

	loans := make([]*types.Loan, len(list))
	for i, l := range list {
		loans[i] = mapLoan(l)
	}

	return &types.LoanListResult{
		Items: loans,
		Pagination: types.Pagination{
			Page:       page,
			PageSize:   pageSize,
			Total:      total,
			TotalPages: int32(math.Ceil(float64(total) / float64(pageSize))),
		},
	}, nil
}

func (s *Service) GetLoan(ctx context.Context, loanId string) (*types.Loan, error) {
	id, err := utils.ParseUUID(loanId)
	if err != nil {
		return nil, err
	}

	loan, err := s.repo.GetLoanByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, utils.NewNotFoundError("loan not found", err)
		}

		return nil, utils.NewInternalError("failed to fetch loan", err)
	}

	return mapLoan(loan), nil
}

func (s *Service) CreateDeposit(ctx context.Context, input types.CreateDepositInput) error {
	id, err := utils.ParseUUID(input.LoanID)
	if err != nil {
		return err
	}

	loan, err := s.repo.GetLoanByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return utils.NewNotFoundError("loan not found", err)
		}

		return utils.NewInternalError("failed to fetch loan", err)
	}

	amountPaid, err := strconv.ParseFloat(input.Amount, 64)
	if err != nil {
		return utils.NewInvalidArgumentError("invalid amount")
	}

	monthlyRepayment := loan.MonthlyPayment
	mp, err := strconv.ParseFloat(monthlyRepayment.String, 64)
	if err != nil {
		return utils.NewInvalidArgumentError("invalid monthly interest")
	}

	if mp <= 0 {
		return utils.NewInvalidArgumentError("invalid loan repayment config")
	}

	monthsPaid := int32(amountPaid / mp)
	if monthsPaid > loan.TermMonths.Int32 {
		monthsPaid = loan.TermMonths.Int32
	}

	args := repository.CreateDepositParams{
		Status:  "pending",
		Type:    loan.LoanType,
		Months:  monthsPaid,
		Amount:  input.Amount,
		Receipt: input.Receipt,
		LoanID:  loan.ID,
		UserID:  loan.UserID,
		Email:   loan.Email,
	}

	_, err = s.repo.CreateDeposit(ctx, args)
	if err != nil {
		return utils.NewInternalError("failed to create deposit", err)
	}

	return nil
}

func (s *Service) GetDeposit(ctx context.Context, depositID string) (*types.Deposit, error) {
	id, err := utils.ParseUUID(depositID)
	if err != nil {
		return nil, err
	}

	d, err := s.repo.GetDepositByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, utils.NewNotFoundError("deposit not found", err)
		}

		return nil, utils.NewInternalError("failed to fetch deposit", err)
	}

	return mapDeposit(d), nil
}

func (s *Service) GetDeposits(ctx context.Context, page int32, pageSize int32) (*types.DepositListResult, error) {
	if page < 1 {
		page = 1
	}

	if pageSize < 1 {
		pageSize = 10
	}

	offset := (page - 1) * pageSize

	total, err := s.repo.CountDeposits(ctx)
	if err != nil {
		return nil, utils.NewInternalError("unable to count deposits", err)
	}

	list, err := s.repo.GetDeposits(ctx, repository.GetDepositsParams{
		Limit:  pageSize,
		Offset: offset,
	})
	if err != nil {
		return nil, utils.NewInternalError("unable to get deposits", err)
	}

	d := make([]*types.Deposit, len(list))
	for i, l := range list {
		d[i] = mapDeposit(l)
	}

	return &types.DepositListResult{
		Items: d,
		Pagination: types.Pagination{
			Page:       page,
			PageSize:   pageSize,
			Total:      total,
			TotalPages: int32(math.Ceil(float64(total) / float64(pageSize))),
		},
	}, nil
}

func (s *Service) ManageDeposit(ctx context.Context, req types.ManageInput) error {
	return nil
}

func (s *Service) LoginStaff(ctx context.Context, email string, password string) (*types.StaffLoginResult, error) {
	staff, err := s.repo.GetStaffByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, utils.NewNotFoundError(fmt.Sprintf("staff not found with email: %s", email), err)
		}
		return nil, utils.NewInternalError(fmt.Sprintf("unable to find staff with email: %s", email), err)
	}

	if !staff.AccountEnabled.Bool {
		return nil, utils.NewPermissionDeniedError("your account is temporarily disabled")
	}

	err = utils.CheckPassword(password, staff.PasswordHash)
	if err != nil {
		return nil, utils.NewUnathenticatedError("incorrect password")
	}

	err = s.repo.UpdateLastLoginStaff(ctx, staff.ID)
	if err != nil {
		s.logger.Warn("failed updating last login", zap.Error(err))
	}

	token, _, err := s.maker.CreateToken(staff.Email, staff.ID.String(), string(staff.Role.UserRole), staff.AccountEnabled.Bool, 12*time.Hour)
	if err != nil {
		return nil, utils.NewInternalError("unable to create access token", err)
	}

	return &types.StaffLoginResult{
		Token:          token,
		ID:             staff.ID.String(),
		Email:          staff.Email,
		FullName:       staff.FullName,
		Role:           string(staff.Role.UserRole),
		AccountEnabled: staff.AccountEnabled.Bool,
		LastLogin:      staff.LastLoginAt.Time,
		CreatedAt:      staff.CreatedAt.Time,
	}, nil
}

func (s *Service) CreateLoanType(ctx context.Context, req types.LoanTypeRequest) error {
	_, err := s.repo.CreateLoanType(ctx, repository.CreateLoanTypeParams{
		Name: req.Name,
		Rate: req.Rate,
	})

	if err != nil {
		return utils.NewInternalError("unable to create loan type", err)
	}

	return nil
}

func (s *Service) UpdateLoanType(ctx context.Context, req types.UpdateLoanTypeRequest) error {
	id, err := utils.ParseUUID(req.ID)
	if err != nil {
		return err
	}

	_, err = s.repo.GetLoanTypeByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return utils.NewNotFoundError("loan type not found", err)
		}

		return utils.NewInternalError("failed to fetch loan type", err)
	}

	args := repository.UpdateLoanTypeParams{
		ID:   id,
		Name: sql.NullString{Valid: req.Name != "", String: req.Name},
		Rate: sql.NullString{Valid: req.Rate != "", String: req.Rate},
	}

	if err := s.repo.UpdateLoanType(ctx, args); err != nil {
		return utils.NewInternalError("unable to update loan type", err)
	}

	return nil
}

func (s *Service) CreateStaff(ctx context.Context, req types.CreateStaffRequest) error {
	password := utils.RandomString(12)

	hash, err := utils.HashPassword(password)
	if err != nil {
		return utils.NewInternalError("unable to hash staff password", err)
	}

	args := repository.CreateStaffParams{
		Email:        req.Email,
		PasswordHash: hash,
		FullName:     req.FullName,
		Role:         repository.NullUserRole{Valid: req.Role != "", UserRole: repository.UserRole(req.Role)},
	}

	if _, err := s.repo.CreateStaff(ctx, args); err != nil {
		return utils.NewInternalError("unable to create staff", err)
	}

	loginURL := fmt.Sprintf("%s/auth/login-employee", os.Getenv("FRONTEND_URL"))
	content := fmt.Sprintf(`
        <div style="font-family: sans-serif; line-height: 1.6; color: #333;">
            <h2 style="color: #D61F28;">Welcome to the Team</h2>
            <p>Your staff account has been created. Please use the credentials below to log in:</p>
            <ul style="list-style: none; padding: 0;">
                <li><strong>Email:</strong> %s</li>
                <li><strong>Temporary Password:</strong> <code>%s</code></li>
            </ul>
            <p>Please log in and update your password immediately:</p>
            <a href="%s" style="background-color: #D61F28; color: white; padding: 10px 20px; text-decoration: none; border-radius: 5px; font-weight: bold;">
                Login to Platform
            </a>
            <p style="font-size: 12px; color: #888; margin-top: 20px;">
                If you did not request this account, please contact your administrator.
            </p>
        </div>
    `, req.Email, password, loginURL)

	if err := s.mailer.SendMail("Welcome Mail", content, []string{req.Email}, nil, nil, nil); err != nil {
		s.logger.Error("unable to send email to staff", zap.Error(err))
	}

	return nil
}

func (s *Service) GetStaffs(ctx context.Context, page int32, pageSize int32) (*types.StaffListResult, error) {
	if page < 1 {
		page = 1
	}

	if pageSize < 1 {
		pageSize = 10
	}

	offset := (page - 1) * pageSize

	total, err := s.repo.CountStaffs(ctx)
	if err != nil {
		return nil, utils.NewInternalError("unable to count staffs", err)
	}

	list, err := s.repo.GetAllStaff(ctx, repository.GetAllStaffParams{
		Limit:  pageSize,
		Offset: offset,
	})
	if err != nil {
		return nil, utils.NewInternalError("unable to get loans", err)
	}

	staffs := make([]*types.Staff, len(list))
	for i, l := range list {
		staffs[i] = &types.Staff{
			ID:             l.ID.String(),
			FullName:       l.FullName,
			Email:          l.Email,
			AccountEnabled: l.AccountEnabled.Bool,
			Role:           types.Role(l.Role.UserRole),
			CreatedAt:      l.CreatedAt.Time,
		}
	}

	return &types.StaffListResult{
		Items: staffs,
		Pagination: types.Pagination{
			Page:       page,
			PageSize:   pageSize,
			Total:      total,
			TotalPages: int32(math.Ceil(float64(total) / float64(pageSize))),
		},
	}, nil
}

func (s *Service) GetStaff(ctx context.Context, staffID string) (*types.Staff, error) {
	id, err := utils.ParseUUID(staffID)
	if err != nil {
		return nil, err
	}

	staff, err := s.repo.GetStaffByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, utils.NewNotFoundError("staff not found", err)
		}

		return nil, utils.NewInternalError("failed to fetch staff", err)
	}

	return &types.Staff{
		ID:             staff.ID.String(),
		FullName:       staff.FullName,
		Email:          staff.Email,
		AccountEnabled: staff.AccountEnabled.Bool,
		Role:           types.Role(staff.Role.UserRole),
		CreatedAt:      staff.CreatedAt.Time,
	}, nil
}

func (s *Service) UpdateStaff(ctx context.Context, req types.UpdateStaffRequest) error {
	id, err := utils.ParseUUID(req.ID)
	if err != nil {
		return err
	}

	_, err = s.repo.GetStaffByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return utils.NewNotFoundError("staff not found", err)
		}

		return utils.NewInternalError("failed to fetch staff", err)
	}

	args := repository.UpdateStaffParams{
		ID:       id,
		FullName: sql.NullString{Valid: req.FullName != "", String: req.FullName},
		Role:     repository.NullUserRole{Valid: req.Role != "", UserRole: repository.UserRole(req.Role)},
	}

	if err := s.repo.UpdateStaff(ctx, args); err != nil {
		return utils.NewInternalError("unable to update staff", err)
	}

	return nil
}

func (s *Service) ManageStaff(ctx context.Context, req types.StaffAction) error {
	id, err := utils.ParseUUID(req.ID)
	if err != nil {
		return err
	}

	staff, err := s.repo.GetStaffByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return utils.NewNotFoundError("staff not found", err)
		}
		return utils.NewInternalError("failed to fetch staff", err)
	}

	action := strings.ToLower(strings.TrimSpace(req.Action))

	var targetState bool

	switch action {
	case "enable":
		targetState = true
	case "disable":
		targetState = false
	default:
		return utils.NewInvalidArgumentError(
			"action must be one of: enable, disable",
		)
	}

	if staff.AccountEnabled.Bool == targetState {
		return nil
	}

	if err := s.repo.UpdateStaff(ctx, repository.UpdateStaffParams{
		ID:             staff.ID,
		AccountEnabled: sql.NullBool{Valid: true, Bool: targetState},
	}); err != nil {
		return utils.NewInternalError("failed to update staff account status", err)
	}

	return nil
}

func (s *Service) sendLoanEmail(loan repository.Loan, status string, actionLink string) error {
	title := fmt.Sprintf("Loan Update: %s", strings.ToUpper(status))

	summary := fmt.Sprintf(`
        <div style="background-color: #211E1B; padding: 15px; border-radius: 8px; margin: 20px 0;">
            <table style="width: 100%%; border-collapse: collapse; color: #E6E1DC;">
                <tr><td style="padding: 5px 0; color: #8C8176;">Reference:</td><td style="text-align: right;">%s</td></tr>
                <tr><td style="padding: 5px 0; color: #8C8176;">Loan Type:</td><td style="text-align: right;">%s</td></tr>
                <tr><td style="padding: 5px 0; color: #8C8176;">Amount:</td><td style="text-align: right;">₦%s</td></tr>
                <tr><td style="padding: 5px 0; color: #8C8176;">Status:</td><td style="text-align: right; color: #E6A15C;">%s</td></tr>
            </table>
        </div>`, loan.ID, loan.LoanType, utils.FormatWithCommas(loan.PrincipalAmount), strings.ToUpper(status))

	body := fmt.Sprintf("<p>Hello %s,</p><p>Your loan application status has been updated to <strong>%s</strong>.</p>", loan.BorrowerName, status)
	body += summary

	action := ""
	if actionLink != "" {
		action = fmt.Sprintf(`<a href="%s" style="background-color: #E6A15C; color: #1A1816; padding: 12px 24px; text-decoration: none; border-radius: 8px; font-weight: bold; display: inline-block;">View Application</a>`, actionLink)
	}

	content := strings.Replace(utils.EmailTemplate, "{{TITLE}}", title, 1)
	content = strings.Replace(content, "{{BODY}}", body, 1)
	content = strings.Replace(content, "{{ACTION}}", action, 1)

	return s.mailer.SendMail("Loan Application", content, []string{loan.Email}, nil, nil, nil)
}

func mapLoan(loan repository.Loan) *types.Loan {
	return &types.Loan{
		ID:                 loan.ID.String(),
		LoanType:           loan.LoanType,
		PrincipalAmount:    loan.PrincipalAmount,
		InterestRate:       loan.InterestRate,
		TermMonths:         nullInt32(loan.TermMonths),
		MonthlyPayment:     nullString(loan.MonthlyPayment),
		AdminFee:           loan.AdminFee,
		TotalInterest:      nullString(loan.TotalInterest),
		TotalRepayment:     nullString(loan.TotalRepayment),
		TotalRepaid:        loan.TotalRepaid,
		TotalUnpaid:        loan.TotalUnpaid,
		NumberOfRepayments: loan.NumberOfRepayments,
		Status:             loan.Status,
		DueDate:            nullTime(loan.DueDate),
		ApprovedDate:       nullTime(loan.ApprovedDate),
		NextPaymentDate:    nullTime(loan.NextPaymentDate),
		Collateral:         loan.Collateral,
		BorrowerName:       loan.BorrowerName,
		Email:              loan.Email,
		GuarantorName:      nullString(loan.GuarantorName),
		GuarantorEmail:     nullString(loan.GuarantorEmail),
		GuarantorPhone:     nullString(loan.GuarantorPhone),
		GuarantorIppisNo:   nullString(loan.GuarantorIppisNo),
		BankName:           nullString(loan.BankName),
		AccountNumber:      nullString(loan.AccountNumber),
		AccountHolder:      nullString(loan.AccountHolder),
		BVN:                nullString(loan.Bvn),
		Occupation:         nullString(loan.Occupation),
		EmployerName:       nullString(loan.EmployerName),
		EmployerAddress:    nullString(loan.EmployerAddress),
		EmployerPhone:      nullString(loan.EmployerPhone),
		IppisNo:            nullString(loan.IppisNo),
		Statement:          nullString(loan.Statement),
		AdminFeeReceipt:    nullString(loan.AdminFeeReceipt),
		CollateralDocument: nullString(loan.CollateralDocument),
		LoanInterest:       nullString(loan.LoanInterest),
		UserID:             loan.UserID.String(),
		CreatedAt:          loan.CreatedAt,
		UpdatedAt:          loan.UpdatedAt,
	}
}

func mapDeposit(deposit repository.Deposit) *types.Deposit {
	return &types.Deposit{
		ID:        deposit.ID.String(),
		TxID:      deposit.TxID.String(),
		Status:    deposit.Status,
		Type:      deposit.Type,
		Months:    deposit.Months,
		Amount:    deposit.Amount,
		Receipt:   deposit.Receipt,
		LoanID:    deposit.LoanID.String(),
		UserID:    deposit.UserID.String(),
		Email:     deposit.Email,
		CreatedAt: deposit.CreatedAt,
		UpdatedAt: deposit.UpdatedAt,
	}
}

func nullString(v sql.NullString) string {
	if v.Valid {
		return v.String
	}
	return ""
}

func nullInt32(v sql.NullInt32) int32 {
	if v.Valid {
		return v.Int32
	}
	return 0
}

func nullTime(v sql.NullTime) *time.Time {
	if v.Valid {
		return &v.Time
	}
	return nil
}
