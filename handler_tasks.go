package main

import (
	"database/sql"
	"net/http"
	"time"

	"github.com/nazifbara/kanban-api/internal/apierrors"
	"github.com/nazifbara/kanban-api/internal/database"
	"github.com/nazifbara/kanban-api/internal/tasks"
	"github.com/nazifbara/kanban-api/internal/utils"
)

func (s *server) handlerDeleteTask(w http.ResponseWriter, r *http.Request) {
	taskID, err := utils.GetIdFromPath(r, "taskID")
	if err != nil {
		respondWithError(r.Context(), w, apierrors.FromErr(http.StatusBadRequest, err))
		return
	}
	err = s.store.execTx(r.Context(), func(q *database.Queries) error {
		task, err := q.GetTaskForShare(r.Context(), taskID)
		if err != nil {
			return apierrors.FromDBErr(err)
		}
		err = q.ShiftTasksAfter(
			r.Context(),
			database.ShiftTasksAfterParams{After: task.Position, Delta: -1, ColumnID: task.ColumnID},
		)
		if err != nil {
			return apierrors.FromDBErr(err)
		}
		if err := q.DeleteTask(r.Context(), taskID); err != nil {
			return apierrors.FromDBErr(err)
		}
		return nil
	})
	if err != nil {
		respondWithError(r.Context(), w, err)
		return
	}

	w.WriteHeader(http.StatusOK)
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

	patch, err := tasks.PrepareTaskPatch(param)
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
				after := min(newPosition, task.Position)
				before := max(newPosition, task.Position)
				delta := 0
				if newPosition > task.Position {
					// if the new position is foward we need to shift
					// tasks in between backward, including the task that was
					// holding newPosition
					delta = -1
					before++
				} else {
					// just the other way around
					delta = 1
					after--
				}
				err = qtx.ShiftTasksBetween(
					r.Context(),
					database.ShiftTasksBetweenParams{
						After:    after,
						Before:   before,
						Delta:    int32(delta),
						ColumnID: newColumnId,
					},
				)
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
	respondWithJSON(w, http.StatusOK, tasks.DBToTask(updatedTask))
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
	dbTasks, err := s.store.GetColumnTasks(
		r.Context(),
		columnID,
	)
	if err != nil {
		respondWithError(r.Context(), w, apierrors.FromDBErr(err))
		return
	}
	respondWithJSON(w, http.StatusOK, tasks.DBToTaskSlice(dbTasks))
}

func (s *server) handlerCreateTask(w http.ResponseWriter, r *http.Request) {
	params, err := decodeJSONBody[tasks.CreateParam](r)
	if err != nil {
		respondWithError(r.Context(), w, malformedBodyErr)
		return
	}
	if err := tasks.ValidateCreateParam(params); err != nil {
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
		err = qtx.ShiftTasksAfter(
			r.Context(),
			database.ShiftTasksAfterParams{
				After:    int32(params.Position),
				Delta:    1,
				ColumnID: dbColumn.ID,
			},
		)
		if err != nil {
			return apierrors.FromDBErr(err)
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
			return apierrors.FromDBErr(err)
		}
		return nil
	})
	if err != nil {
		respondWithError(r.Context(), w, err)
		return
	}
	respondWithJSON(w, http.StatusCreated, tasks.DBToTask(dbTask))
}
