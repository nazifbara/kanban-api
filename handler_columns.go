package main

import (
	"database/sql"
	"net/http"

	"github.com/google/uuid"
	"github.com/nazifbara/kanban-api/internal/apierrors"
	"github.com/nazifbara/kanban-api/internal/columns"
	"github.com/nazifbara/kanban-api/internal/database"
	"github.com/nazifbara/kanban-api/internal/utils"
)

func prepareColumnPatch(params columns.PatchParams) database.UpdateColumnParams {
	var patch database.UpdateColumnParams
	if params.Title != nil {
		patch.Title = sql.NullString{String: *params.Title, Valid: true}
	}
	if params.Description != nil {
		patch.Description = sql.NullString{String: *params.Description, Valid: true}
	}
	if params.Position != nil {
		patch.Position = sql.NullInt32{Int32: *params.Position, Valid: true}
	}
	return patch
}

func (s *server) handlerPatchColumn(w http.ResponseWriter, r *http.Request) {
	columnID, err := utils.GetIdFromPath(r, "columnID")
	if err != nil {
		s.respondWithError(r.Context(), w, apierrors.FromErr(http.StatusBadRequest, err))
	}
	patchParams, err := decodeJSONBody[columns.PatchParams](r)
	if err != nil {
		s.respondWithError(r.Context(), w, malformedBodyErr)
		return
	}
	var dbColumn database.Column
	err = s.store.execTx(r.Context(), func(q *database.Queries) error {
		oldColumn, err := q.GetColumn(r.Context(), columnID)
		if err != nil {
			return apierrors.FromDBErr(err)
		}
		count, err := q.CountColumns(r.Context(), oldColumn.BoardID)
		if err != nil {
			return apierrors.FromDBErr(err)
		}
		if err := columns.ValidatePatch(patchParams, utils.Ptr(int(count-1))); err != nil {
			return apierrors.FromErr(http.StatusBadRequest, err)
		}
		patch := prepareColumnPatch(patchParams)
		patch.ID = columnID

		if patchParams.Position != nil && *patchParams.Position != oldColumn.Position {
			after := min(*patchParams.Position, oldColumn.Position)
			before := max(*patchParams.Position, oldColumn.Position)
			delta := 0
			if patchParams.Position != nil && *patchParams.Position > oldColumn.Position {
				// if the new position is foward we need to shift
				// columns in between backward, including the column that was
				// holding patchParams.Position
				delta = -1
				before++
			} else {
				// just the other way around
				delta = 1
				after--
			}
			err = q.ShiftColumnsBetween(
				r.Context(),
				database.ShiftColumnsBetweenParams{
					After:   after,
					Before:  before,
					Delta:   int32(delta),
					BoardID: oldColumn.BoardID,
				},
			)
		}
		if err != nil {
			return err
		}
		dbColumn, err = q.UpdateColumn(r.Context(), patch)
		if err != nil {
			return apierrors.FromDBErr(err)
		}
		return nil
	})
	if err != nil {
		s.respondWithError(r.Context(), w, err)
		return
	}
	s.respondWithJSON(w, http.StatusOK, columns.DBToColumn(dbColumn))
}

func (s *server) handlerDeleteColumn(w http.ResponseWriter, r *http.Request) {
	columnID, err := utils.GetIdFromPath(r, "columnID")
	if err != nil {
		s.respondWithError(r.Context(), w, apierrors.FromErr(http.StatusBadRequest, err))
		return
	}
	err = s.store.execTx(r.Context(), func(q *database.Queries) error {
		dbColumn, err := q.GetColumn(r.Context(), columnID)
		if err != nil {
			return apierrors.FromDBErr(err)
		}
		err = q.ShiftColumnsFrom(
			r.Context(),
			database.ShiftColumnsFromParams{Start: dbColumn.Position + 1, Delta: -1, BoardID: dbColumn.BoardID},
		)
		if err != nil {
			return apierrors.FromDBErr(err)
		}
		err = q.DeleteColumn(r.Context(), columnID)
		if err != nil {
			return apierrors.FromDBErr(err)
		}
		return nil
	})
	if err != nil {
		s.respondWithError(r.Context(), w, err)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (s *server) handlerBoardColumns(w http.ResponseWriter, r *http.Request) {
	param, err := decodeJSONBody[columns.ColumnBoardID](r)
	if err != nil {
		s.respondWithError(r.Context(), w, malformedBodyErr)
		return
	}
	var dbColumns []database.Column
	err = s.store.execTx(r.Context(), func(q *database.Queries) error {
		board, err := q.GetBoard(r.Context(), param.BoardID)
		if err != nil {
			return apierrors.FromDBErr(err)
		}
		dbColumns, err = q.GetColumns(r.Context(), board.ID)
		if err != nil {
			return apierrors.FromDBErr(err)
		}
		return nil
	})
	if err != nil {
		s.respondWithError(r.Context(), w, err)
		return
	}
	s.respondWithJSON(w, 200, columns.DBToColumnSlice(dbColumns))
}

func (s *server) handlerCreateColumn(w http.ResponseWriter, r *http.Request) {
	params, err := decodeJSONBody[columns.CreateParams](r)
	if err != nil {
		s.respondWithError(r.Context(), w, malformedBodyErr)
		return
	}
	if params.BoardID == uuid.Nil {
		s.respondWithError(r.Context(), w, apierrors.New(http.StatusBadRequest, "body.board_id is required"))
		return
	}
	var dbColumn database.Column
	err = s.store.execTx(r.Context(), func(qtx *database.Queries) error {
		count, err := qtx.CountColumns(r.Context(), params.BoardID)
		if err != nil {
			return apierrors.FromDBErr(err)
		}
		if err := columns.ValidateColumn(params, int(count)); err != nil {
			return apierrors.FromErr(http.StatusBadRequest, err)
		}
		_, err = qtx.GetBoard(r.Context(), params.BoardID)
		if err != nil {
			return apierrors.FromDBErr(err)
		}
		err = qtx.ShiftColumnsFrom(r.Context(), database.ShiftColumnsFromParams{
			Start:   params.Position,
			Delta:   1,
			BoardID: params.BoardID,
		})
		if err != nil {
			return apierrors.FromDBErr(err)
		}
		dbColumn, err = qtx.CreateColumn(
			r.Context(),
			database.CreateColumnParams{
				BoardID:  params.BoardID,
				Title:    params.Title,
				Position: params.Position,
			},
		)
		if err != nil {
			return apierrors.FromDBErr(err)
		}
		return nil
	})
	if err != nil {
		s.respondWithError(r.Context(), w, err)
		return
	}
	s.respondWithJSON(w, http.StatusCreated, columns.DBToColumn(dbColumn))
}
