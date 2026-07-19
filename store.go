package main

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/nazifbara/kanban-api/internal/database"
)

type store struct {
	*database.Queries
	db *sql.DB
}

func (s *store) execTx(ctx context.Context, fn func(*database.Queries) error) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	qtx := s.WithTx(tx)
	err = fn(qtx)
	if err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			return fmt.Errorf("tx error: %w, rollback error: %w", err, rbErr)
		}
		return err
	}
	return tx.Commit()
}
