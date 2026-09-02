package main

import (
	"database/sql"
	"net/http"

	"github.com/google/uuid"
	"github.com/nazifbara/kanban-api/internal/apierrors"
	"github.com/nazifbara/kanban-api/internal/database"
	"github.com/nazifbara/kanban-api/internal/tasks"
	"github.com/nazifbara/kanban-api/internal/utils"
)

func (s *server) handlerGetBoardTasks(w http.ResponseWriter, r *http.Request) {
	ctxBoard, err := getBoardFromCtx(r.Context())
	if err != nil {
		respondWithError(r.Context(), w, err)
		return
	}
	dbTaskSlice, err := s.store.GetBoardTasks(r.Context(), ctxBoard.ID)
	if err != nil {
		respondWithError(r.Context(), w, apierrors.FromDBErr(err))
		return
	}
	respondWithJSON(r.Context(), w, http.StatusOK, tasks.DBToTaskSlice(dbTaskSlice))
}

func (s *server) handlerDeleteTask(w http.ResponseWriter, r *http.Request) {
	ctxBoard, err := getBoardFromCtx(r.Context())
	if err != nil {
		respondWithError(r.Context(), w, err)
		return
	}
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
		if task.BoardID != ctxBoard.ID {
			return apierrors.New(http.StatusNotFound, "task not found in the board")
		}
		err = q.ShiftTasksFrom(
			r.Context(),
			database.ShiftTasksFromParams{Start: task.Position + 1, Delta: -1, ColumnID: task.ColumnID},
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
	ctxBoard, err := getBoardFromCtx(r.Context())
	if err != nil {
		respondWithError(r.Context(), w, err)
		return
	}
	param, err := decodeJSONBody[tasks.PatchParam](r)
	if err != nil {
		respondWithError(r.Context(), w, malformedBodyErr)
		return
	}

	taskID, err := utils.GetIdFromPath(r, "taskID")
	if err != nil {
		respondWithError(r.Context(), w, apierrors.FromErr(http.StatusBadRequest, err))
		return
	}

	var updatedTask database.Task
	err = s.store.execTx(r.Context(), func(q *database.Queries) error {
		task, err := q.GetTaskForUpdate(r.Context(), taskID)
		if err != nil {
			return apierrors.FromDBErr(err)
		}
		if task.BoardID != ctxBoard.ID {
			return apierrors.New(http.StatusNotFound, "task not found in the board")
		}
		var destinationColumn database.Column
		if param.ColumnID != nil && *param.ColumnID == uuid.Nil {
			return apierrors.New(http.StatusBadRequest, "invalid column ID")
		}
		if param.ColumnID != nil && *param.ColumnID != task.ColumnID {
			destinationColumn, err = q.GetColumn(r.Context(), *param.ColumnID)
			if err != nil {
				return apierrors.FromDBErr(err)
			}
			if destinationColumn.BoardID != task.BoardID {
				return apierrors.New(http.StatusNotFound, "column not found")
			}

			if param.Position == nil {
				param.Position = new(int32)
			}

			c, err := q.CountTasks(r.Context(), *param.ColumnID)
			if err != nil {
				return apierrors.FromDBErr(err)
			}
			count := utils.Ptr(int(c))

			// this is where the update begin on this branch
			// so it's time to validate the request body
			if err := tasks.ValidatePatchParam(param, count); err != nil {
				return apierrors.FromErr(http.StatusBadRequest, err)
			}
			err = tasks.ChangeTaskColumn(r.Context(), q, task, *param.ColumnID, *param.Position)
			if err != nil {
				return apierrors.FromDBErr(err)
			}
		} else if param.Position != nil && *param.Position != int32(task.Position) {
			after := min(*param.Position, task.Position)
			before := max(*param.Position, task.Position)
			delta := 0
			if *param.Position > task.Position {
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
			count, err := q.CountTasks(r.Context(), task.ColumnID)
			if err != nil {
				return apierrors.FromDBErr(err)
			}

			if err := tasks.ValidatePatchParam(param, utils.Ptr(int(count-1))); err != nil {
				return apierrors.FromErr(http.StatusBadRequest, err)
			}
			err = q.ShiftTasksBetween(
				r.Context(),
				database.ShiftTasksBetweenParams{
					After:    after,
					Before:   before,
					Delta:    int32(delta),
					ColumnID: task.ColumnID,
				},
			)
			if err != nil {
				return apierrors.FromDBErr(err)
			}
		} else {
			if err := tasks.ValidatePatchParam(param, nil); err != nil {
				return apierrors.FromErr(http.StatusBadRequest, err)
			}
		}

		patch := tasks.PrepareTaskPatch(param)
		patch.ID = taskID
		updatedTask, err = q.UpdateTask(r.Context(), patch)
		if err != nil {
			return apierrors.FromDBErr(err)
		}
		return nil
	})
	if err != nil {
		respondWithError(r.Context(), w, err)
		return
	}
	respondWithJSON(r.Context(), w, http.StatusOK, tasks.DBToTask(updatedTask))
}

func (s *server) handlerColumnTasks(w http.ResponseWriter, r *http.Request) {
	ctxColumn, err := getColumnFromCtx(r.Context())
	if err != nil {
		respondWithError(r.Context(), w, err)
		return
	}
	dbTasks, err := s.store.GetColumnTasks(
		r.Context(),
		ctxColumn.ID,
	)
	if err != nil {
		respondWithError(r.Context(), w, apierrors.FromDBErr(err))
		return
	}
	respondWithJSON(r.Context(), w, http.StatusOK, tasks.DBToTaskSlice(dbTasks))
}

func (s *server) handlerCreateTask(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserIDFromCtx(r.Context())
	if err != nil {
		respondWithError(r.Context(), w, err)
		return
	}
	ctxColumn, err := getColumnFromCtx(r.Context())
	if err != nil {
		respondWithError(r.Context(), w, err)
		return
	}
	params, err := decodeJSONBody[tasks.CreateParam](r)
	if err != nil {
		respondWithError(r.Context(), w, malformedBodyErr)
		return
	}
	var dbTask database.Task
	err = s.store.execTx(r.Context(), func(q *database.Queries) error {
		count, err := q.CountTasks(
			r.Context(),
			ctxColumn.ID,
		)
		if err != nil {
			return apierrors.FromDBErr(err)
		}
		if err := tasks.ValidateCreateParam(params, int(count)); err != nil {
			return apierrors.FromErr(http.StatusBadRequest, err)
		}
		err = q.ShiftTasksFrom(
			r.Context(),
			database.ShiftTasksFromParams{
				// include the previous task holding params.Position
				Start:    int32(params.Position),
				Delta:    1,
				ColumnID: ctxColumn.ID,
			},
		)
		if err != nil {
			return apierrors.FromDBErr(err)
		}
		dbTask, err = q.CreateTask(
			r.Context(),
			database.CreateTaskParams{
				BoardID:     ctxColumn.BoardID,
				ColumnID:    ctxColumn.ID,
				Description: sql.NullString{String: params.Description, Valid: true},
				Title:       params.Title,
				Position:    params.Position,
				CreatorID:   userID,
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
	respondWithJSON(r.Context(), w, http.StatusCreated, tasks.DBToTask(dbTask))
}
