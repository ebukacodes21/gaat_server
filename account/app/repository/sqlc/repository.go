package repository

import (
	"context"
	"database/sql"
)

type DatabaseContract interface {
	Querier
	CreateUserAccountTx(ctx context.Context, args CreateAccountTxParams) (User, error)
	ManageDepositTx(ctx context.Context, args ManageDepositTxParams) error
}

type Repository struct {
	*Queries
	db *sql.DB
}

func NewRepository(db *sql.DB) DatabaseContract {
	return &Repository{db: db, Queries: New(db)}
}

func (r *Repository) execTx(ctx context.Context, fn func(tx *Queries) error) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}

	q := New(tx)
	err = fn(q)
	if err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			return rbErr
		}
		return err
	}

	return tx.Commit()
}
