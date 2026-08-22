package main

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"slices"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	utils "github.com/nazifbara/kanban-api/internal"
	"github.com/nazifbara/kanban-api/internal/apierrors"
	"github.com/nazifbara/kanban-api/internal/database"
)

type Task struct {
	ID          uuid.UUID `json:"id"`
	BoardID     uuid.UUID `json:"board_id"`
	ColumnID    uuid.UUID `json:"column_id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Position    int       `json:"position"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type CreateTaskParam struct {
	ColumnID    uuid.UUID `json:"column_id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Position    int       `json:"position"`
}

type UpdateTaskParam struct {
	ColumnID    *uuid.UUID `json:"column_id"`
	Title       *string    `json:"title"`
	Description *string    `json:"description"`
	Position    *int32     `json:"position"`
}

func prepareTaskPatch(param UpdateTaskParam) (database.UpdateTaskParams, error) {
	var patch database.UpdateTaskParams
	if param.ColumnID != nil {
		patch.ColumnID = uuid.NullUUID{UUID: *param.ColumnID, Valid: true}
	}
	if param.Title != nil {
		if *param.Title == "" {
			return patch, errors.New("body.title can't be empty")
		}
		patch.Title = sql.NullString{String: *param.Title, Valid: true}
	}
	if param.Description != nil {
		patch.Description = sql.NullString{String: *param.Description, Valid: true}
	}
	if param.Position != nil {
		patch.Position = sql.NullInt32{Int32: *param.Position, Valid: true}
	}
	return patch, nil
}

func (s *server) handlerUpdateTask(w http.ResponseWriter, r *http.Request) {
	param, err := decodeJSONBody[UpdateTaskParam](r)
	if err != nil {
		respondWithError(r.Context(), w, malformedBodyErr)
		return
	}

	taskID, err := utils.GetIdFromPath(r, "taskID")
	if err != nil {
		respondWithError(r.Context(), w, apierrors.FromErr(http.StatusBadRequest, err))
		return
	}

	task, err := s.store.GetTask(r.Context(), taskID)
	if err != nil {
		respondWithError(r.Context(), w, apierrors.FromDBErr(err))
		return
	}

	patch, err := prepareTaskPatch(param)
	if err != nil {
		respondWithError(r.Context(), w, apierrors.FromErr(http.StatusBadRequest, err))
		return
	}

	newColumnId := task.ColumnID
	newPosition := task.Position
	if patch.ColumnID.Valid {
		newColumnId = patch.ColumnID.UUID
	}
	if patch.Position.Valid {
		newPosition = int32(patch.Position.Int32)
	}

	const maxRetries = 3
	var updatedTask database.Task
	for range maxRetries {
		if !patch.Position.Valid {
			newPosition = task.Position
		}
		err = s.store.execTx(r.Context(), func(qtx *database.Queries) error {
			var err error
			if newColumnId != task.ColumnID {
				column, err := s.store.GetColumnForShare(r.Context(), patch.ColumnID.UUID)
				if err != nil {
					err = apierrors.FromDBErr(err)
				}
				if column.BoardID != task.BoardID {
					err = apierrors.New(http.StatusNotFound, "column not found")
				}
				if !patch.Position.Valid {
					newPosition = 0
				}
				err = changeTaskColumn(r.Context(), qtx, task, newColumnId, int(newPosition))
			} else if newPosition != int32(task.Position) {
				positionTaskParam := PositionTaskParam{
					queries:  qtx,
					columnID: newColumnId,
					dbTask:   task,
					position: int(newPosition),
				}
				err = positionTask(r.Context(), positionTaskParam)
			}

			if err != nil {
				return err
			}

			patch.ID = taskID
			updatedTask, err = s.store.UpdateTask(r.Context(), patch)
			if err != nil {
				return apierrors.FromDBErr(err)
			}
			return nil
		})
		if err == nil {
			break
		}
		if !apierrors.IsDBRetryable(err) {
			break
		}

		task, err = s.store.GetTask(r.Context(), taskID)
		if err != nil {
			err = apierrors.FromDBErr(err)
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	if err != nil {
		respondWithError(r.Context(), w, err)
		return
	}
	respondWithJSON(w, http.StatusOK, dbToTask(updatedTask))
}

func changeTaskColumn(ctx context.Context, q *database.Queries, task database.Task, columnID uuid.UUID, newPosition int) error {
	oldColumnTasks, err := q.GetColumnTasksForUpdate(
		ctx,
		task.ColumnID,
	)
	if err != nil {
		return err
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
	positionTaskParam := PositionTaskParam{
		queries:  q,
		columnID: columnID,
		dbTask:   task,
		position: newPosition,
	}
	if err := positionTask(ctx, positionTaskParam); err != nil {
		return err
	}
	return nil
}

func validateCreateTaskParam(param CreateTaskParam) error {
	var errs []error
	if param.ColumnID == uuid.Nil {
		errs = append(errs, errors.New("body.column_id is required"))
	}
	if param.Title == "" {
		errs = append(errs, errors.New("body.title is required"))
	}
	if utf8.RuneCountInString(param.Title) > 255 {
		errs = append(errs, errors.New("body.title cannot exceed 255 characters"))
	}
	return errors.Join(errs...)
}

func (s *server) handlerColumnTasks(w http.ResponseWriter, r *http.Request) {
	columnID, err := utils.GetIdFromPath(r, "columnID")
	if err != nil {
		respondWithError(r.Context(), w, apierrors.FromErr(http.StatusBadRequest, err))
		return
	}
	_, err = s.store.GetColumn(r.Context(), columnID)
	if err != nil {
		respondWithError(r.Context(), w, apierrors.FromDBErr(err))
		return
	}
	tasks, err := s.store.GetColumnTasks(
		r.Context(),
		columnID,
	)
	if err != nil {
		respondWithError(r.Context(), w, apierrors.FromDBErr(err))
		return
	}
	respondWithJSON(w, http.StatusOK, dbToTaskSlice(tasks))
}

func dbToTaskSlice(dbTasks []database.Task) []Task {
	tasks := []Task{}
	for _, dbTask := range dbTasks {
		tasks = append(tasks, dbToTask(dbTask))
	}
	return tasks
}

func (s *server) handlerCreateTask(w http.ResponseWriter, r *http.Request) {
	params, err := decodeJSONBody[CreateTaskParam](r)
	if err != nil {
		respondWithError(r.Context(), w, malformedBodyErr)
		return
	}
	if err := validateCreateTaskParam(params); err != nil {
		respondWithError(r.Context(), w, err)
		return
	}

	var dbTask database.Task
	err = s.store.execTx(r.Context(), func(qtx *database.Queries) error {
		var err error
		dbColumn, err := qtx.GetColumnForShare(r.Context(), params.ColumnID)
		if err != nil {
			return apierrors.FromDBErr(err)
		}
		positionTaskParam := PositionTaskParam{
			queries:  qtx,
			columnID: dbColumn.ID,
			dbTask:   dbTask,
			position: params.Position,
		}
		if err := positionTask(r.Context(), positionTaskParam); err != nil {
			return err
		}
		dbTask, err = qtx.CreateTask(
			r.Context(),
			database.CreateTaskParams{
				BoardID:     dbColumn.BoardID,
				ColumnID:    dbColumn.ID,
				Description: sql.NullString{String: params.Description, Valid: true},
				Title:       params.Title,
				Position:    int32(params.Position),
			},
		)
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		respondWithError(r.Context(), w, err)
		return
	}
	respondWithJSON(w, http.StatusCreated, dbToTask(dbTask))
}

type PositionTaskParam struct {
	queries  *database.Queries
	columnID uuid.UUID
	dbTask   database.Task
	position int
}

func positionTask(ctx context.Context, param PositionTaskParam) error {
	destinationTasks, err := param.queries.GetColumnTasksForUpdate(
		ctx,
		param.columnID,
	)
	if err != nil {
		return apierrors.FromDBErr(err)
	}
	oldPosition := slices.IndexFunc(destinationTasks, func(t database.Task) bool {
		return t.ID == param.dbTask.ID
	})
	if !utils.IsPositionInRange(param.position, len(destinationTasks), oldPosition == -1) {
		return apierrors.New(http.StatusBadRequest, "position out of range")
	}

	stopIdx := len(destinationTasks)
	if oldPosition != -1 {
		stopIdx = oldPosition
	}

	if oldPosition == -1 {
		for i := param.position; i < len(destinationTasks); i++ {
			task := destinationTasks[i]
			task.Position++
			err = param.queries.UpdateTaskPosition(ctx, database.UpdateTaskPositionParams{
				ID:       task.ID,
				Position: task.Position,
			})
			if err != nil {
				return apierrors.FromDBErr(err)
			}
		}
	} else if oldPosition > param.position {
		for i := param.position; i < stopIdx; i++ {
			task := destinationTasks[i]
			task.Position++
			err = param.queries.UpdateTaskPosition(ctx, database.UpdateTaskPositionParams{
				ID:       task.ID,
				Position: task.Position,
			})
			if err != nil {
				return apierrors.FromDBErr(err)
			}
		}
	} else {
		for i := param.position; i > stopIdx; i-- {
			task := destinationTasks[i]
			task.Position--
			err = param.queries.UpdateTaskPosition(ctx, database.UpdateTaskPositionParams{
				ID:       task.ID,
				Position: task.Position,
			})
			if err != nil {
				return apierrors.FromDBErr(err)
			}
		}
	}

	return nil
}

func dbToTask(dbTask database.Task) Task {
	return Task{
		ID:          dbTask.ID,
		BoardID:     dbTask.BoardID,
		ColumnID:    dbTask.ColumnID,
		Title:       dbTask.Title,
		Description: dbTask.Description.String,
		CreatedAt:   dbTask.CreatedAt,
		UpdatedAt:   dbTask.UpdatedAt,
		Position:    int(dbTask.Position),
	}
}
