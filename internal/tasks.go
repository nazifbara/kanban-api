package tasks

import (
	"context"

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
