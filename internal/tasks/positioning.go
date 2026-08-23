package tasks

import (
	"context"
	"database/sql"
	"net/http"
	"slices"

	"github.com/google/uuid"
	"github.com/nazifbara/kanban-api/internal/apierrors"
	"github.com/nazifbara/kanban-api/internal/database"
	"github.com/nazifbara/kanban-api/internal/utils"
)

func PositionTask(ctx context.Context, q *database.Queries, taskID, columnID uuid.UUID, newPosition int) error {
	destinationTasks, err := q.GetColumnTasksForUpdate(ctx, columnID)
	if err != nil {
		return apierrors.FromDBErr(err)
	}
	err = utils.PositionItem(ctx, utils.PositionParam[database.Task]{
		Items:    destinationTasks,
		ItemID:   func(t database.Task) uuid.UUID { return t.ID },
		ItemPos:  func(t database.Task) int { return int(t.Position) },
		TargetID: taskID,
		Position: newPosition,
		UpdateItem: func(ctx context.Context, id uuid.UUID, pos int32) error {
			err := q.UpdateTaskPosition(ctx, database.UpdateTaskPositionParams{
				ID:       id,
				Position: pos,
			})
			if err != nil {
				return err
			}
			return nil
		},
	})
	if err != nil {
		return err
	}
	return nil
}

func ChangeTaskColumn(ctx context.Context, q *database.Queries, task database.Task, columnID uuid.UUID, newPosition int) error {
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
	err = PositionTask(ctx, q, task.ID, columnID, newPosition)
	if err != nil {
		return err
	}
	return nil
}
