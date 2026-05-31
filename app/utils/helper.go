package utils

import (
	"fmt"
	"math/rand"
	"net/mail"
	"net/url"
	"strconv"
	"strings"
	"unicode"

	"github.com/google/uuid"
)

const alpha = "abcdefghijklmnopqrstuvwxyz"

func RandomString(n int) string {
	var sb strings.Builder

	for i := 0; i < n; i++ {
		char := alpha[rand.Intn(len(alpha))]
		sb.WriteByte(char)
	}

	return sb.String()
}

func GenerateCode() string {
	code := rand.Intn(900000) + 100000
	return fmt.Sprintf("%06d", code)
}

func ParseUUID(v string) (uuid.UUID, error) {
	id, err := uuid.Parse(v)
	if err != nil {
		return uuid.UUID{}, NewInvalidArgumentError(fmt.Sprintf("invalid UUID: %s", v))
	}

	return id, nil
}

func ParseInt32(value string) int32 {
	n, err := strconv.ParseInt(value, 10, 32)
	if err != nil {
		return 0
	}
	return int32(n)
}

type LoanCalculation struct {
	MonthlyPayment  float64
	MonthlyInterest float64
	TotalInterest   float64
	TotalRepayment  float64
}

func CalculateLoan(amount float64, term int, interestRate float64) LoanCalculation {
	monthlyInterest := amount * interestRate
	totalInterest := monthlyInterest * float64(term)
	totalRepayment := amount + totalInterest

	return LoanCalculation{
		MonthlyPayment:  totalRepayment / float64(term),
		MonthlyInterest: monthlyInterest,
		TotalInterest:   totalInterest,
		TotalRepayment:  totalRepayment,
	}
}

func ParseURL(v string) error {
	u, err := url.ParseRequestURI(v)
	if err != nil {
		return NewInvalidArgumentError(
			fmt.Sprintf("invalid URL: %s", v),
		)
	}

	if u.Scheme == "" || u.Host == "" {
		return NewInvalidArgumentError(
			fmt.Sprintf("invalid URL: %s", v),
		)
	}

	return nil
}

func ParseEmail(email string) error {
	email = strings.TrimSpace(email)

	if _, err := mail.ParseAddress(email); err != nil {
		return NewInvalidArgumentError("invalid email address")
	}

	return nil
}

func ParsePhone(phone string) error {
	phone = strings.TrimSpace(phone)

	if len(phone) != 11 {
		return NewInvalidArgumentError("phone number must be 11 digits")
	}

	for _, r := range phone {
		if !unicode.IsDigit(r) {
			return NewInvalidArgumentError("phone number must contain only digits")
		}
	}

	return nil
}
