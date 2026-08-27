package tasks

import (
	"context"

	"github.com/google/uuid"
	"github.com/nazifbara/kanban-api/internal/database"
)

func ChangeTaskColumn(ctx context.Context, q *database.Queries, task database.Task, columnID uuid.UUID, newPosition int32) error {
	err := q.ShiftTasksFrom(
		ctx,
		database.ShiftTasksFromParams{
			ColumnID: task.ColumnID,
			Delta:    -1,
			Start:    task.Position + 1,
		},
	)
	if err != nil {
		return err
	}
	err = q.ShiftTasksFrom(
		ctx,
		database.ShiftTasksFromParams{
			ColumnID: columnID,
			Delta:    1,
			Start:    newPosition,
		},
	)
	if err != nil {
		return err
	}
	return nil
}
