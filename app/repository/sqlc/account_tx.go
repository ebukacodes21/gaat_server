package repository

import (
	"app/utils"
	"context"
	"fmt"
	"log"

	pg "github.com/lib/pq"
)

type CreateAccountTxParams struct {
	CreateUserParams
	AfterCreate func(u User) error
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
