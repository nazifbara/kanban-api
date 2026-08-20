package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	utils "github.com/nazifbara/kanban-api/internal"
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
	Title       *string
	Description *string `json:"description"`
	Position    *int32  `json:"position"`
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
		respondWithError(r.Context(), w, http.StatusBadRequest, malformedBodyErr)
		return
	}

	taskID, err := utils.GetIdFromPath(r, "taskID")
	if err != nil {
		respondWithError(r.Context(), w, http.StatusBadRequest, errors.New("invalid task ID"))
		return
	}

	task, err := s.store.GetTask(r.Context(), taskID)
	if err != nil {
		respondFromDBErr(r.Context(), w, err)
		return
	}

	patch, err := prepareTaskPatch(param)
	if err != nil {
		respondWithError(r.Context(), w, http.StatusBadRequest, err)
		return
	}

	changeColumn := false
	newColumnId := task.ColumnID
	newPosition := task.Position
	var destinationTasks []database.Task

	if patch.ColumnID.Valid {
		column, err := s.store.GetColumnById(r.Context(), patch.ColumnID.UUID)
		if err != nil {
			respondFromDBErr(r.Context(), w, err)
			return
		}
		if column.BoardID != task.BoardID {
			respondWithError(r.Context(), w, http.StatusNotFound, errors.New("column not found"))
			return
		}
		changeColumn = patch.ColumnID.UUID != task.ColumnID
		newColumnId = patch.ColumnID.UUID
	}
	if changeColumn {
		destinationTasks, err = s.store.GetColumnTasks(
			r.Context(),
			newColumnId,
		)
		if err != nil {
			respondFromDBErr(r.Context(), w, err)
			return
		}
		if !patch.Position.Valid {
			newPosition = int32(len(destinationTasks))
		}
	}
	if patch.Position.Valid {
		if destinationTasks == nil {
			destinationTasks, err = s.store.GetColumnTasks(
				r.Context(),
				task.ColumnID,
			)
			if err != nil {
				respondFromDBErr(r.Context(), w, err)
				return
			}
		}
		numOfTasks := len(destinationTasks)
		if numOfTasks == 0 && patch.Position.Int32 > 0 {
			respondWithError(r.Context(), w, http.StatusBadRequest, fmt.Errorf("task position out of range [0, 0]"))
			return
		} else if numOfTasks > 0 && (int(patch.Position.Int32) >= numOfTasks || patch.Position.Int32 < 0) {
			respondWithError(r.Context(), w, http.StatusBadRequest, fmt.Errorf("task position out of range [0, %d]", numOfTasks-1))
			return
		}
		newPosition = int32(patch.Position.Int32)
	}

	var updatedTask database.Task
	err = s.store.execTx(r.Context(), func(q *database.Queries) error {
		var err error
		if changeColumn {
			err = changeTaskColumn(r.Context(), q, task, destinationTasks, patch.ColumnID.UUID, int(newPosition))
		} else if newPosition != int32(task.Position) {
			err = positionTask(r.Context(), q, destinationTasks, task, int(newPosition))
		}

		if err != nil {
			return err
		}

		patch.ID = taskID
		updatedTask, err = s.store.UpdateTask(r.Context(), patch)
		if err != nil {
			return err
		}
		return nil
	})

	if err != nil {
		respondWith500(r.Context(), w, err)
		return
	}
	respondWithJSON(w, http.StatusOK, dbToTask(updatedTask))
}

func changeTaskColumn(ctx context.Context, q *database.Queries, task database.Task, newColumnTasks []database.Task, columnID uuid.UUID, newPosition int) error {
	oldColumnTasks, err := q.GetColumnTasks(
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
		return errors.New("Task not found in old column")
	}
	for i := taskIndex + 1; i < len(oldColumnTasks); i++ {
		t := oldColumnTasks[i]
		_, err := q.UpdateTask(ctx, database.UpdateTaskParams{ID: t.ID, Position: sql.NullInt32{Int32: t.Position - 1, Valid: true}})
		if err != nil {
			return err
		}
	}
	task.Position = int32(newPosition)
	err = positionTask(ctx, q, newColumnTasks, task, newPosition)
	if err != nil {
		return err
	}
	return nil
}

func validateCreateTaskParam(param CreateTaskParam) error {
	var err []error
	if param.ColumnID == uuid.Nil {
		err = append(err, errors.New("body.column_id is required"))
	}
	if param.Title == "" {
		err = append(err, errors.New("body.title is required"))
	}
	if utf8.RuneCountInString(param.Title) > 255 {
		err = append(err, errors.New("body.title cannot excedd 255 characters"))
	}
	return errors.Join(err...)
}

func (s *server) handlerColumnTasks(w http.ResponseWriter, r *http.Request) {
	columnID, err := utils.GetIdFromPath(r, "columnID")
	if err != nil {
		respondWithError(r.Context(), w, http.StatusBadRequest, err)
		return
	}
	_, err = s.store.GetColumnById(r.Context(), columnID)
	if err != nil {
		respondFromDBErr(r.Context(), w, err)
		return
	}
	tasks, err := s.store.GetColumnTasks(
		r.Context(),
		columnID,
	)
	if err != nil {
		respondFromDBErr(r.Context(), w, err)
		return
	}
	respondWithJSON(w, http.StatusOK, dbToTaskSlice(tasks))
}

func dbToTaskSlice(dbTasks []database.Task) []Task {
	var tasks []Task
	for _, dbTask := range dbTasks {
		tasks = append(tasks, dbToTask(dbTask))
	}
	return tasks
}

func (s *server) handlerCreateTask(w http.ResponseWriter, r *http.Request) {
	params, err := decodeJSONBody[CreateTaskParam](r)
	if err != nil {
		respondWithError(r.Context(), w, http.StatusBadRequest, malformedBodyErr)
		return
	}
	if err := validateCreateTaskParam(params); err != nil {
		respondWithError(r.Context(), w, http.StatusBadRequest, err)
		return
	}
	dbColumn, err := s.store.GetColumnById(r.Context(), params.ColumnID)
	if err != nil {
		respondFromDBErr(r.Context(), w, err)
		return
	}
	existingTasks, err := s.store.GetColumnTasks(r.Context(), dbColumn.ID)
	if err != nil {
		respondFromDBErr(r.Context(), w, err)
		return
	}
	var dbTask database.Task
	err = s.store.execTx(r.Context(), func(qtx *database.Queries) error {
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
		if err := positionTask(r.Context(), qtx, existingTasks, dbTask, params.Position); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		respondWith500(r.Context(), w, err)
		return
	}
	respondWithJSON(w, http.StatusCreated, dbToTask(dbTask))
}

func positionTask(context context.Context, q *database.Queries, columnTasks []database.Task, dbTask database.Task, newPosition int) error {
	oldPosition := slices.IndexFunc(columnTasks, func(t database.Task) bool {
		return t.ID == dbTask.ID
	})
	stopIdx := len(columnTasks)
	if oldPosition != -1 {
		stopIdx = oldPosition
	}
	var err error
	if oldPosition == -1 {
		for i := newPosition; i < len(columnTasks); i++ {
			task := columnTasks[i]
			task.Position++
			err = q.UpdateTaskPosition(context, database.UpdateTaskPositionParams{
				ID:       task.ID,
				Position: task.Position,
			})
			if err != nil {
				break
			}
		}
	} else if oldPosition > newPosition {
		for i := newPosition; i < stopIdx; i++ {
			task := columnTasks[i]
			task.Position++
			err = q.UpdateTaskPosition(context, database.UpdateTaskPositionParams{
				ID:       task.ID,
				Position: task.Position,
			})
			if err != nil {
				break
			}
		}
	} else {
		for i := newPosition; i > stopIdx; i-- {
			task := columnTasks[i]
			task.Position--
			err = q.UpdateTaskPosition(context, database.UpdateTaskPositionParams{
				ID:       task.ID,
				Position: task.Position,
			})
			if err != nil {
				break
			}
		}
	}

	return err
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
