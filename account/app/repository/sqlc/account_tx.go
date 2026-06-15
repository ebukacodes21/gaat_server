package repository

import (
	"app/utils"
	"context"
	"database/sql"
	"fmt"
	"log"
	"math"
	"strconv"
	"strings"
	"time"

	pg "github.com/lib/pq"
)

type CreateAccountTxParams struct {
	CreateUserParams
	AfterCreate func(u User) error
}

type ManageDepositTxParams struct {
	DepositID string
	Action    string
}

func (r *Repository) CreateUserAccountTx(ctx context.Context, args CreateAccountTxParams) (User, error) {
	var u User
	err := r.execTx(ctx, func(tx *Queries) error {
		var err error
		u, err = tx.CreateUser(ctx, args.CreateUserParams)
		if err != nil {
			if pgErr, ok := err.(*pg.Error); ok {
				switch pgErr.Code.Name() {
				case "unique_violation":
					return utils.NewAlreadyExistsError(fmt.Sprintf("account with email address %s already exists", args.Email), err)
				}
			}

			return utils.NewInternalError("unable to register account", err)
		}

		if args.AfterCreate != nil {
			if err := args.AfterCreate(u); err != nil {
				log.Print("unable to send mail")
				return utils.NewInternalError("unable to dispatch mail", err)
			}
		}

		return nil
	})

	return u, err
}

func (r *Repository) ManageDepositTx(ctx context.Context, args ManageDepositTxParams) (Deposit, error) {
	var deposit Deposit
	err := r.execTx(ctx, func(tx *Queries) error {
		id, err := utils.ParseUUID(args.DepositID)
		if err != nil {
			return err
		}

		log.Print(id, "========================")

		// get deposit
		deposit, err = tx.GetDepositByID(ctx, id)
		if err != nil {
			return utils.NewInternalError("failed to fetch deposit", err)
		}

		current := strings.ToLower(strings.TrimSpace(deposit.Status))

		// idempotency
		if current == args.Action {
			log.Print(current, "no op ========================")
			return nil // no-op
		}

		terminal := map[string]bool{
			"rejected": true,
		}

		if terminal[current] {
			return utils.NewInvalidArgumentError(
				"loan is already in a terminal state",
			)
		}

		// rules
		allowed := map[string][]string{
			"pending":   {"forwarded"},
			"forwarded": {"approved", "rejected"},
			"approved":  {},
			"rejected":  {},
		}

		ok := false
		for _, v := range allowed[current] {
			if v == args.Action {
				ok = true
				break
			}
		}

		if !ok {
			return utils.NewInvalidArgumentError(
				fmt.Sprintf("cannot transition deposit from %s to %s", current, args.Action),
			)
		}

		log.Print("updating.....")
		log.Print(args.Action)

		// update deposit status
		err = tx.UpdateDepositStatus(ctx, UpdateDepositStatusParams{
			ID:     id,
			Status: args.Action,
		})
		if err != nil {
			return utils.NewInternalError("failed to update deposit", err)
		}

		deposit.Status = args.Action

		// only process repayment logic when approved
		if args.Action != "approved" {
			return nil
		}

		// get loan
		loan, err := tx.GetLoanByID(ctx, deposit.LoanID)
		if err != nil {
			return utils.NewInternalError("failed to fetch loan", err)
		}

		// parse values
		depositAmount, err := strconv.ParseFloat(deposit.Amount, 64)
		if err != nil {
			return utils.NewInternalError("invalid deposit amount", err)
		}

		totalRepaid, err := strconv.ParseFloat(loan.TotalRepaid, 64)
		if err != nil {
			return utils.NewInternalError("invalid total repaid", err)
		}

		totalUnpaid, err := strconv.ParseFloat(loan.TotalUnpaid, 64)
		if err != nil {
			return utils.NewInternalError("invalid total unpaid", err)
		}

		monthlyPayment, err := strconv.ParseFloat(loan.MonthlyPayment.String, 64)
		if err != nil {
			return utils.NewInternalError("invalid monthly payment", err)
		}

		tracker, err := strconv.ParseFloat(loan.AmountPaidTowardsNextInstallment, 64)
		if err != nil {
			return utils.NewInternalError("invalid installment tracker", err)
		}

		// apply repayment
		totalRepaid += depositAmount
		totalUnpaid -= depositAmount

		if totalUnpaid < 0 {
			totalUnpaid = 0
		}

		tracker += depositAmount

		// compute completed installments
		completed := 0

		if monthlyPayment > 0 {
			completed = int(tracker / monthlyPayment)

			if completed > 0 {
				tracker = math.Mod(tracker, monthlyPayment)
			}
		}

		// update repayment count
		repayments := loan.NumberOfRepayments + int32(completed)

		// update next payment date
		nextPayment := loan.NextPaymentDate

		if completed > 0 {
			if nextPayment.Valid {
				nextPayment.Time = nextPayment.Time.AddDate(0, completed, 0)
			} else {
				nextPayment = sql.NullTime{
					Valid: true,
					Time:  time.Now().AddDate(0, completed, 0),
				}
			}
		}

		// handle full repayment
		status := loan.Status

		if totalUnpaid == 0 {
			status = "repaid"
			tracker = 0
			nextPayment = sql.NullTime{Valid: false}
		}

		log.Print(UpdateLoanRepaymentParams{
			ID:                               loan.ID,
			TotalRepaid:                      fmt.Sprintf("%.2f", totalRepaid),
			TotalUnpaid:                      fmt.Sprintf("%.2f", totalUnpaid),
			NumberOfRepayments:               repayments,
			AmountPaidTowardsNextInstallment: fmt.Sprintf("%.2f", tracker),
			NextPaymentDate:                  nextPayment,
			Status:                           status,
		})

		// persist loan update
		return tx.UpdateLoanRepayment(ctx, UpdateLoanRepaymentParams{
			ID:                               loan.ID,
			TotalRepaid:                      fmt.Sprintf("%.2f", totalRepaid),
			TotalUnpaid:                      fmt.Sprintf("%.2f", totalUnpaid),
			NumberOfRepayments:               repayments,
			AmountPaidTowardsNextInstallment: fmt.Sprintf("%.2f", tracker),
			NextPaymentDate:                  nextPayment,
			Status:                           status,
		})
	})
	return deposit, err
}
