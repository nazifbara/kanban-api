package main

import (
	"database/sql"
	"errors"
	"net/http"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/nazifbara/kanban-api/internal/apierrors"
	"github.com/nazifbara/kanban-api/internal/database"
	"github.com/nazifbara/kanban-api/internal/tasks"
	"github.com/nazifbara/kanban-api/internal/utils"
)

func prepareTaskPatch(param tasks.UpdateParam) (database.UpdateTaskParams, error) {
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
	param, err := decodeJSONBody[tasks.UpdateParam](r)
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
			var column database.Column
			if newColumnId != task.ColumnID {
				column, err = qtx.GetColumnForShare(r.Context(), patch.ColumnID.UUID)
				if err != nil {
					return apierrors.FromDBErr(err)
				}
				if column.BoardID != task.BoardID {
					return apierrors.New(http.StatusNotFound, "column not found")
				}
				if !patch.Position.Valid {
					newPosition = 0
				}
				err = tasks.ChangeTaskColumn(r.Context(), qtx, task, newColumnId, int(newPosition))
			} else if newPosition != int32(task.Position) {
				err = tasks.PositionTask(r.Context(), qtx, task.ID, newColumnId, int(newPosition))
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

func validateCreateTaskParam(param tasks.CreateParam) error {
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

func dbToTaskSlice(dbTasks []database.Task) []tasks.Task {
	tasks := []tasks.Task{}
	for _, dbTask := range dbTasks {
		tasks = append(tasks, dbToTask(dbTask))
	}
	return tasks
}

func (s *server) handlerCreateTask(w http.ResponseWriter, r *http.Request) {
	params, err := decodeJSONBody[tasks.CreateParam](r)
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
		err = tasks.PositionTask(r.Context(), qtx, uuid.Nil, dbColumn.ID, params.Position)
		if err != nil {
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

func dbToTask(dbTask database.Task) tasks.Task {
	return tasks.Task{
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
