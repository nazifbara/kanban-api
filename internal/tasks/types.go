package tasks

import (
	"time"

	"github.com/google/uuid"
	"github.com/nazifbara/kanban-api/internal/database"
)

type Task struct {
	ID          uuid.UUID `json:"id"`
	CreatorID   uuid.UUID `json:"creator_id"`
	BoardID     uuid.UUID `json:"board_id"`
	ColumnID    uuid.UUID `json:"column_id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Position    int       `json:"position"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type CreateParam struct {
	ColumnID    uuid.UUID `json:"column_id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Position    int32     `json:"position"`
}

type PatchParam struct {
	ColumnID    *uuid.UUID `json:"column_id"`
	Title       *string    `json:"title"`
	Description *string    `json:"description"`
	Position    *int32     `json:"position"`
}

func DBToTaskSlice(dbTasks []database.Task) []Task {
	tasks := []Task{}
	for _, dbTask := range dbTasks {
		tasks = append(tasks, DBToTask(dbTask))
	}
	return tasks
}

func DBToTask(dbTask database.Task) Task {
	return Task{
		ID:          dbTask.ID,
		BoardID:     dbTask.BoardID,
		ColumnID:    dbTask.ColumnID,
		Title:       dbTask.Title,
		Description: dbTask.Description.String,
		CreatorID:   dbTask.CreatorID,
		CreatedAt:   dbTask.CreatedAt,
		UpdatedAt:   dbTask.UpdatedAt,
		Position:    int(dbTask.Position),
	}
}
