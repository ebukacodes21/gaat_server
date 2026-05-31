package handler

import (
	"app/service"
	"app/types"
	"app/utils"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/bytedance/gopkg/util/logger"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type GaatServer struct {
	R        *gin.Engine
	service  service.GaatService
	maker    utils.TokenMaker
	s3Bucket *s3.Client
}

func NewGaatServer(svc service.GaatService, maker utils.TokenMaker) GaatServer {
	cfg, err := config.LoadDefaultConfig(context.Background(), config.WithRegion(os.Getenv("APP_AWS_REGION")))
	if err != nil {
		logger.Error("unable to load AWS configuration", zap.Any("err", err))
		panic(err)
	}

	s3Bucket := s3.NewFromConfig(cfg)
	r := gin.Default()

	gs := GaatServer{
		R:        r,
		service:  svc,
		s3Bucket: s3Bucket,
	}

	r.POST("/api/register", gs.RegisterAccount)
	r.POST("/api/verify", gs.VerifyAccount)
	r.POST("/api/resend", gs.ResendOTP)
	r.POST("/api/login", gs.LoginAccount)
	r.POST("/api/login-staff", gs.LoginStaff)
	r.POST("/api/forgot", gs.ForgotPassword)
	r.POST("/api/reset", gs.ResetPassword)
	r.GET("/api/loan-types", gs.ListLoanTypes)

	// ---------- AUTH ROUTES ----------
	authRoutes := r.Group("/").Use(utils.AuthMiddleware(maker))

	// USER
	authRoutes.POST("/api/upload", utils.RequireRole("user"), gs.UploadToS3)
	authRoutes.GET("/api/user/query", utils.RequireRole("user"), gs.QueryUser)
	authRoutes.GET("/api/user", utils.RequireRole("user"), gs.User)
	authRoutes.PATCH("/api/user/update-password", utils.RequireRole("user"), gs.UpdatePassword)
	authRoutes.PATCH("/api/user/update-account", utils.RequireRole("user"), gs.UpdateAccount)
	authRoutes.GET("/api/loans/user", utils.RequireRole("user"), gs.UserLoans)
	authRoutes.GET("/api/deposits/user", utils.RequireRole("user"), gs.UserDeposits)
	authRoutes.POST("/api/loans/request", utils.RequireRole("user"), gs.RequestLoan)
	authRoutes.POST("/api/deposits/request", utils.RequireRole("user"), gs.CreateDeposit)

	// STAFF+
	authRoutes.GET("/api/loans", utils.RequireRole("staff"), gs.ListLoans)
	authRoutes.GET("/api/deposits", utils.RequireRole("staff"), gs.ListDeposits)
	authRoutes.GET("/api/loan", utils.RequireRole("user"), gs.Loan)
	authRoutes.GET("/api/deposit", utils.RequireRole("user"), gs.Deposit)

	// SUPERVISOR+
	authRoutes.PATCH("/api/loans/manage", utils.RequireRole("supervisor"), gs.ManageLoan)
	authRoutes.PATCH("/api/deposits/manage", utils.RequireRole("supervisor"), gs.ManageDeposit)
	authRoutes.GET("/api/users", utils.RequireRole("supervisor"), gs.ListUsers)
	authRoutes.PATCH("/api/users/manage", utils.RequireRole("supervisor"), gs.ManageUser)

	// ADMIN ONLY
	authRoutes.POST("/api/loan-types/create", utils.RequireRole("admin"), gs.CreateLoanType)
	authRoutes.PATCH("/api/loan-types/update", utils.RequireRole("admin"), gs.UpdateLoanType)

	authRoutes.POST("/api/staffs/create", utils.RequireRole("admin"), gs.CreateStaff)
	authRoutes.PATCH("/api/staffs/update", utils.RequireRole("admin"), gs.UpdateStaff)
	authRoutes.POST("/api/staffs/manage", utils.RequireRole("admin"), gs.ManageStaff)
	authRoutes.GET("/api/staffs", utils.RequireRole("admin"), gs.ListStaffs)
	authRoutes.GET("/api/staff", utils.RequireRole("admin"), gs.Staff)

	return gs
}

func (gs *GaatServer) RegisterAccount(c *gin.Context) {
	var body types.CreateUserInput

	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("invalid request payload: %v", err)})
		return
	}

	user, err := gs.service.RegisterAccount(c.Request.Context(), &body)
	if err != nil {
		code, errMsg := utils.TranslateDomainError(err)
		c.JSON(code, gin.H{"error": errMsg})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "account created successfully",
		"data":    user,
	})
}

func (gs *GaatServer) VerifyAccount(c *gin.Context) {
	var body types.VerifyInput
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("invalid request payload: %v", err)})
		return
	}

	err := gs.service.VerifyUser(c.Request.Context(), body.Email, body.Code)
	if err != nil {
		code, errMsg := utils.TranslateDomainError(err)
		c.JSON(code, gin.H{"error": errMsg})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "account verification successfully",
	})
}

func (gs *GaatServer) ForgotPassword(c *gin.Context) {
	var body types.ForgotInput

	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("invalid request payload: %v", err),
		})
		return
	}

	result, err := gs.service.Forgot(c.Request.Context(), body.Email)
	if err != nil {
		code, msg := utils.TranslateDomainError(err)
		c.JSON(code, gin.H{"error": msg})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": result.Message,
	})
}

func (gs *GaatServer) LoginAccount(c *gin.Context) {
	var body types.LoginInput

	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("invalid request payload: %v", err),
		})
		return
	}

	result, err := gs.service.LoginUser(c.Request.Context(), body.Email, body.Password)
	if err != nil {
		code, msg := utils.TranslateDomainError(err)
		c.JSON(code, gin.H{"error": msg})
		return
	}

	c.SetCookie("auth_token", result.Token, 60*60*12, "/", "", true, true)

	c.JSON(http.StatusOK, gin.H{
		"message": "login successful",
		"data":    result,
	})
}

func (gs *GaatServer) LoginStaff(c *gin.Context) {
	var body types.LoginInput

	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("invalid request payload: %v", err),
		})
		return
	}

	result, err := gs.service.LoginStaff(c.Request.Context(), body.Email, body.Password)
	if err != nil {
		code, msg := utils.TranslateDomainError(err)
		c.JSON(code, gin.H{"error": msg})
		return
	}

	c.SetCookie("auth_token", result.Token, 60*60*12, "/", "", true, true)

	c.JSON(http.StatusOK, gin.H{
		"message": "login successful",
		"data":    result,
	})
}

func (gs *GaatServer) ResetPassword(c *gin.Context) {
	var body types.ResetInput

	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("invalid request payload: %v", err),
		})
		return
	}

	result, err := gs.service.Reset(c.Request.Context(), body)
	if err != nil {
		code, msg := utils.TranslateDomainError(err)
		c.JSON(code, gin.H{"error": msg})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": result.Message,
	})
}

func (gs *GaatServer) ResendOTP(c *gin.Context) {
	var body types.ForgotInput

	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("invalid request payload: %v", err),
		})
		return
	}

	result, err := gs.service.ResendOTP(c.Request.Context(), body.Email)
	if err != nil {
		code, msg := utils.TranslateDomainError(err)
		c.JSON(code, gin.H{"error": msg})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "OTP resent successfully",
		"data":    result.Data,
	})
}

func (gs *GaatServer) UserLoans(c *gin.Context) {
	payload := c.MustGet(utils.AuthorizationPayloadKey).(*utils.Payload)

	page := utils.ParseInt32(c.DefaultQuery("page", "1"))
	pageSize := utils.ParseInt32(c.DefaultQuery("page_size", "10"))

	loans, err := gs.service.GetUserLoans(c, payload.UserId, page, pageSize)
	if err != nil {
		code, msg := utils.TranslateDomainError(err)
		c.JSON(code, gin.H{"error": msg})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "account loans retrieved",
		"data":    loans,
	})
}

func (gs *GaatServer) ListLoans(c *gin.Context) {
	page := utils.ParseInt32(c.DefaultQuery("page", "1"))
	pageSize := utils.ParseInt32(c.DefaultQuery("page_size", "10"))

	loans, err := gs.service.GetLoans(c, page, pageSize)
	if err != nil {
		code, msg := utils.TranslateDomainError(err)
		c.JSON(code, gin.H{"error": msg})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "loans retrieved",
		"data":    loans,
	})
}

func (gs *GaatServer) Loan(c *gin.Context) {
	loanId := c.Query("loan_id")
	if loanId == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "missing loan id in request query",
		})
		return
	}

	loan, err := gs.service.GetLoan(c, loanId)
	if err != nil {
		code, msg := utils.TranslateDomainError(err)
		c.JSON(code, gin.H{"error": msg})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "loan retrieved",
		"data":    loan,
	})
}

func (gs *GaatServer) Deposit(c *gin.Context) {
	id := c.Query("deposit_id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "missing deposit id in request query",
		})
		return
	}

	loan, err := gs.service.GetDeposit(c, id)
	if err != nil {
		code, msg := utils.TranslateDomainError(err)
		c.JSON(code, gin.H{"error": msg})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "deposit retrieved",
		"data":    loan,
	})
}

func (gs *GaatServer) ListDeposits(c *gin.Context) {
	page := utils.ParseInt32(c.DefaultQuery("page", "1"))
	pageSize := utils.ParseInt32(c.DefaultQuery("page_size", "10"))

	d, err := gs.service.GetDeposits(c, page, pageSize)
	if err != nil {
		code, msg := utils.TranslateDomainError(err)
		c.JSON(code, gin.H{"error": msg})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "deposits retrieved",
		"data":    d,
	})
}

func (gs *GaatServer) ManageLoan(c *gin.Context) {
	_ = c.MustGet(utils.AuthorizationPayloadKey).(*utils.Payload)
	// err := guard(payload.Role, body.Action)
	// if err != nil {
	// 	c.JSON(http.StatusForbidden, gin.H{
	// 		"error": err.Error(),
	// 	})
	// }
	var body types.ManageInput

	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("invalid request payload: %v", err),
		})
		return
	}

	err := gs.service.ManageLoan(c.Request.Context(), body)
	if err != nil {
		code, msg := utils.TranslateDomainError(err)
		c.JSON(code, gin.H{"error": msg})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "loan action dispatched!",
	})
}

func (gs *GaatServer) UserDeposits(c *gin.Context) {
	payload := c.MustGet(utils.AuthorizationPayloadKey).(*utils.Payload)

	page := utils.ParseInt32(c.DefaultQuery("page", "1"))
	pageSize := utils.ParseInt32(c.DefaultQuery("page_size", "10"))

	deposits, err := gs.service.GetUserDeposits(c, payload.UserId, page, pageSize)
	if err != nil {
		code, msg := utils.TranslateDomainError(err)
		c.JSON(code, gin.H{"error": msg})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "account deposits retrieved",
		"data":    deposits,
	})
}

func (gs *GaatServer) UploadToS3(c *gin.Context) {
	payload := c.MustGet(utils.AuthorizationPayloadKey).(*utils.Payload)

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, utils.ErrorRes(errors.New("file is required")))
		return
	}
	defer file.Close()

	userId := payload.UserId

	ext := filepath.Ext(header.Filename)
	ext = strings.ToLower(ext)

	// clean S3 key (no timestamps, no raw filenames)
	fileName := fmt.Sprintf(
		"gaatltd/%s/uploads/%s%s",
		userId,
		uuid.New().String(),
		ext,
	)

	contentType := header.Header.Get("Content-Type")

	allowedTypes := []string{
		"application/pdf",
		"image/jpeg",
		"image/png",
	}

	if !slices.Contains(allowedTypes, contentType) {
		c.JSON(http.StatusBadRequest, utils.ErrorRes(errors.New("unsupported file type")))
		return
	}

	url, err := gs.s3Upload(c, file, fileName, contentType)
	if err != nil {
		log.Printf("failed to upload to S3: %v", err)
		c.JSON(http.StatusInternalServerError, utils.ErrorRes(err))
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "upload successful",
		"url":     url,
	})
}

func (gs *GaatServer) ListUsers(c *gin.Context) {
	page := utils.ParseInt32(c.DefaultQuery("page", "1"))
	pageSize := utils.ParseInt32(c.DefaultQuery("page_size", "10"))

	users, err := gs.service.GetUsers(c, page, pageSize)
	if err != nil {
		code, msg := utils.TranslateDomainError(err)
		c.JSON(code, gin.H{"error": msg})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "account deposits retrieved",
		"data":    users,
	})
}

func (gs *GaatServer) User(c *gin.Context) {
	payload := c.MustGet(utils.AuthorizationPayloadKey).(*utils.Payload)

	user, err := gs.service.GetUser(c, payload.UserId)
	if err != nil {
		code, msg := utils.TranslateDomainError(err)
		c.JSON(code, gin.H{"error": msg})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "user retrieved",
		"data":    user,
	})
}

func (gs *GaatServer) QueryUser(c *gin.Context) {
	userId := c.Query("user_id")
	if userId == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "missing user id in request query",
		})
		return
	}

	user, err := gs.service.GetUser(c, userId)
	if err != nil {
		code, msg := utils.TranslateDomainError(err)
		c.JSON(code, gin.H{"error": msg})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "user retrieved",
		"data":    user,
	})
}

func (gs *GaatServer) CreateLoanType(c *gin.Context) {
	var body types.LoanTypeRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("invalid request payload: %v", err),
		})
		return
	}

	err := gs.service.CreateLoanType(c.Request.Context(), body)
	if err != nil {
		code, msg := utils.TranslateDomainError(err)
		c.JSON(code, gin.H{"error": msg})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "loan type created!",
	})
}

func (gs *GaatServer) UpdateLoanType(c *gin.Context) {
	var body types.UpdateLoanTypeRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("invalid request payload: %v", err),
		})
		return
	}

	err := gs.service.UpdateLoanType(c.Request.Context(), body)
	if err != nil {
		code, msg := utils.TranslateDomainError(err)
		c.JSON(code, gin.H{"error": msg})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "loan type created!",
	})
}

func (gs *GaatServer) RequestLoan(c *gin.Context) {
	payload := c.MustGet(utils.AuthorizationPayloadKey).(*utils.Payload)

	var body types.RequestLoanInput
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("invalid request payload: %v", err)})
		return
	}

	err := gs.service.RequestLoan(c.Request.Context(), payload.UserId, body)
	if err != nil {
		code, errMsg := utils.TranslateDomainError(err)
		c.JSON(code, gin.H{"error": errMsg})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Loan request successful. An Admin will review your request",
	})
}

func (gs *GaatServer) CreateDeposit(c *gin.Context) {
	var body types.CreateDepositInput
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("invalid request payload: %v", err)})
		return
	}

	err := gs.service.CreateDeposit(c.Request.Context(), body)
	if err != nil {
		code, errMsg := utils.TranslateDomainError(err)
		c.JSON(code, gin.H{"error": errMsg})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Deposit request successful. An Admin will review your request",
	})
}

func (gs *GaatServer) ManageDeposit(c *gin.Context) {
	_ = c.MustGet(utils.AuthorizationPayloadKey).(*utils.Payload)
	// err := guard(payload.Role, body.Action)
	// if err != nil {
	// 	c.JSON(http.StatusForbidden, gin.H{
	// 		"error": err.Error(),
	// 	})
	// }
	var body types.ManageInput

	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("invalid request payload: %v", err),
		})
		return
	}

	err := gs.service.ManageDeposit(c.Request.Context(), body)
	if err != nil {
		code, msg := utils.TranslateDomainError(err)
		c.JSON(code, gin.H{"error": msg})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "deposit action dispatched!",
	})
}

func (gs *GaatServer) UpdatePassword(c *gin.Context) {
	payload := c.MustGet(utils.AuthorizationPayloadKey).(*utils.Payload)
	var body types.UpdatePasswordInput

	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("invalid request payload: %v", err),
		})
		return
	}

	result, err := gs.service.UpdatePassword(c.Request.Context(), payload.UserId, body)
	if err != nil {
		code, msg := utils.TranslateDomainError(err)
		c.JSON(code, gin.H{"error": msg})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": result.Message,
	})
}

func (gs *GaatServer) UpdateAccount(c *gin.Context) {
	payload := c.MustGet(utils.AuthorizationPayloadKey).(*utils.Payload)
	var body types.UpdateInput

	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("invalid request payload: %v", err),
		})
		return
	}

	result, err := gs.service.UpdateUser(c.Request.Context(), payload.UserId, body)
	if err != nil {
		code, msg := utils.TranslateDomainError(err)
		c.JSON(code, gin.H{"error": msg})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "account updated successfully",
		"data":    result,
	})
}

func (gs *GaatServer) ManageUser(c *gin.Context) {
	var body types.StaffAction

	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("invalid request payload: %v", err),
		})
		return
	}

	err := gs.service.ManageUser(c.Request.Context(), body)
	if err != nil {
		code, msg := utils.TranslateDomainError(err)
		c.JSON(code, gin.H{"error": msg})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "user action dispatched!",
	})
}

func (gs *GaatServer) ListLoanTypes(c *gin.Context) {
	loanTypes, err := gs.service.ListLoanTypes(c)
	if err != nil {
		code, errMsg := utils.TranslateDomainError(err)
		c.JSON(code, gin.H{"error": errMsg})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": loanTypes,
	})
}

func (gs *GaatServer) CreateStaff(c *gin.Context) {
	var body types.CreateStaffRequest

	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("invalid request payload: %v", err),
		})
		return
	}

	err := gs.service.CreateStaff(c.Request.Context(), body)
	if err != nil {
		code, msg := utils.TranslateDomainError(err)
		c.JSON(code, gin.H{"error": msg})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "staff created successfully!",
	})
}

func (gs *GaatServer) ListStaffs(c *gin.Context) {
	page := utils.ParseInt32(c.DefaultQuery("page", "1"))
	pageSize := utils.ParseInt32(c.DefaultQuery("page_size", "10"))

	staffs, err := gs.service.GetStaffs(c, page, pageSize)
	if err != nil {
		code, msg := utils.TranslateDomainError(err)
		c.JSON(code, gin.H{"error": msg})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "staffs retrieved",
		"data":    staffs,
	})
}

func (gs *GaatServer) Staff(c *gin.Context) {
	staffId := c.Query("staff_id")
	if staffId == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "missing staff id in request query",
		})
		return
	}

	staff, err := gs.service.GetStaff(c, staffId)
	if err != nil {
		code, msg := utils.TranslateDomainError(err)
		c.JSON(code, gin.H{"error": msg})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "staff retrieved",
		"data":    staff,
	})
}

func (gs *GaatServer) UpdateStaff(c *gin.Context) {
	var body types.UpdateStaffRequest

	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("invalid request payload: %v", err),
		})
		return
	}

	err := gs.service.UpdateStaff(c.Request.Context(), body)
	if err != nil {
		code, msg := utils.TranslateDomainError(err)
		c.JSON(code, gin.H{"error": msg})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "staff update successfully!",
	})
}

func (gs *GaatServer) ManageStaff(c *gin.Context) {
	var body types.StaffAction

	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("invalid request payload: %v", err),
		})
		return
	}

	err := gs.service.ManageStaff(c.Request.Context(), body)
	if err != nil {
		code, msg := utils.TranslateDomainError(err)
		c.JSON(code, gin.H{"error": msg})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "staff action dispatched!",
	})
}

func (gs *GaatServer) s3Upload(ctx context.Context, file io.Reader, key string, contentType string) (string, error) {
	input := &s3.PutObjectInput{
		Bucket:      aws.String(os.Getenv("BUCKET")),
		Key:         aws.String(key),
		Body:        file,
		ContentType: aws.String(contentType),
	}

	_, err := gs.s3Bucket.PutObject(ctx, input)
	if err != nil {
		return "", err
	}

	cloudfrontDomain := os.Getenv("CLOUDFRONT_URL")
	if cloudfrontDomain != "" {
		return fmt.Sprintf("%s/%s", cloudfrontDomain, key), nil
	}

	return fmt.Sprintf("https://%s.s3.amazonaws.com/%s", os.Getenv("BUCKET"), key), nil
}

func guard(r, a string) error {
	role := strings.ToLower(strings.TrimSpace(r))
	action := strings.ToLower(strings.TrimSpace(a))

	validRoles := map[string]bool{
		"user":       true,
		"staff":      true,
		"supervisor": true,
		"admin":      true,
	}

	if !validRoles[role] {
		return fmt.Errorf("invalid role")
	}

	switch role {
	case "admin":
		// admin can do everything
		break

	case "staff", "supervisor":
		if action != "forwarded" {
			return fmt.Errorf("forbidden to access route")
		}

	case "user":
		return fmt.Errorf("forbidden to access route")
	}

	return nil
}
