package tasks

import (
	"context"
	"database/sql"
	"net/http"
	"slices"

	"github.com/google/uuid"
	"github.com/nazifbara/kanban-api/internal/apierrors"
	"github.com/nazifbara/kanban-api/internal/database"
)

func ChangeTaskColumn(ctx context.Context, q *database.Queries, task database.Task, columnID uuid.UUID, newPosition int32) error {
	oldColumnTasks, err := q.GetColumnTasksForUpdate(
		ctx,
		task.ColumnID,
	)
	if err != nil {
		return apierrors.FromDBErr(err)
	}
	taskIndex := slices.IndexFunc(oldColumnTasks, func(t database.Task) bool {
		return t.ID == task.ID
	})
	if taskIndex == -1 {
		return apierrors.New(http.StatusNotFound, "task not found in old column")
	}
	for i := taskIndex + 1; i < len(oldColumnTasks); i++ {
		t := oldColumnTasks[i]
		_, err := q.UpdateTask(ctx, database.UpdateTaskParams{ID: t.ID, Position: sql.NullInt32{Int32: t.Position - 1, Valid: true}})
		if err != nil {
			return apierrors.FromDBErr(err)
		}
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
