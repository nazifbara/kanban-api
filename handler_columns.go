package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"time"

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

type ColumnPositionsParams struct {
	BoardID   uuid.UUID   `json:"board_id"`
	Positions []uuid.UUID `json:"positions"`
}

func validateColumnPositionsParams(params ColumnPositionsParams) error {
	var err []error
	if params.BoardID == uuid.Nil {
		err = append(err, errors.New("body.board_id is required"))
	}
	if len(params.Positions) == 0 {
		err = append(err, errors.New("body.positions can't be empty"))
	}
	m := make(map[string]int)
	for _, id := range params.Positions {
		if m[id.String()] == 1 {
			err = append(err, errors.New("body.positions contains duplicated ids"))
			break
		}
		m[id.String()]++
	}
	return errors.Join(err...)
}

func (s *server) handlerUpdateColumnPositions(w http.ResponseWriter, r *http.Request) {
	params, err := decodeJSONBody[ColumnPositionsParams](r)
	if err != nil {
		s.respondWithError(r.Context(), w, malformedBodyErr)
		return
	}
	if err := validateColumnPositionsParams(params); err != nil {
		s.respondWithError(r.Context(), w, apierrors.FromErr(http.StatusBadRequest, err))
		return
	}
	_, err = s.store.GetBoardByID(r.Context(), params.BoardID)
	if err != nil {
		s.respondWithError(r.Context(), w, apierrors.FromDBErr(err))
		return
	}
	boardColumns, err := s.store.GetColumns(r.Context(), params.BoardID)
	if err != nil {
		s.respondWithError(r.Context(), w, apierrors.FromDBErr(err))
		return
	}
	if len(boardColumns) != len(params.Positions) {
		s.respondWithError(r.Context(), w, apierrors.New(http.StatusBadRequest, "positions don't match board columns count"))
		return
	}

	for _, columnID := range params.Positions {
		if !slices.ContainsFunc(boardColumns, func(c database.Column) bool { return c.ID == columnID }) {
			s.respondWithError(r.Context(), w, apierrors.New(http.StatusBadRequest, "at least one column doesn't belong to the board"))
			return
		}
	}
	err = s.store.execTx(r.Context(), func(q *database.Queries) error {
		for position, columnID := range params.Positions {
			if err := q.UpdateColumnPosition(r.Context(), database.UpdateColumnPositionParams{ID: columnID, Position: int32(position)}); err != nil {
				return apierrors.FromDBErr(err)
			}
			oldPosition := slices.IndexFunc(boardColumns, func(c database.Column) bool { return c.ID == columnID })
			if oldPosition == -1 {
				return apierrors.New(http.StatusBadRequest, "can't find column old position")
			}
			boardColumns[oldPosition].Position = int32(position)
			boardColumns[oldPosition].UpdatedAt = time.Now()
		}
		return nil
	})
	if err != nil {
		s.respondWithError(r.Context(), w, err)
		return
	}
	slices.SortFunc(boardColumns, func(a, b database.Column) int { return int(a.Position - b.Position) })
	s.respondWithJSON(w, http.StatusOK, dbToColumnSlice(boardColumns))
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
	oldColumn, err := s.store.GetColumn(r.Context(), columnID)
	if err != nil {
		s.respondWithError(r.Context(), w, apierrors.FromDBErr(err))
		return
	}
	patch := prepareColumnPatch(patchParams)
	patch.ID = columnID
	boardColumns, err := s.store.GetColumns(r.Context(), oldColumn.BoardID)
	if patch.Position.Valid && (int(patch.Position.Int32) >= len(boardColumns) || patch.Position.Int32 < 0) {
		s.respondWithError(r.Context(), w, apierrors.New(http.StatusBadRequest, fmt.Sprintf("column position out of range [0, %d]", len(boardColumns))))
		return
	}
	var column database.Column
	err = s.store.execTx(r.Context(), func(q *database.Queries) error {
		column, err = q.UpdateColumn(r.Context(), patch)
		if err != nil {
			return apierrors.FromDBErr(err)
		}
		err = positionColumn(r.Context(), q, boardColumns, column)
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		s.respondWithError(r.Context(), w, err)
		return
	}
	s.respondWithJSON(w, http.StatusOK, dbToColumn(column))
}

func (s *server) handlerDeleteColumn(w http.ResponseWriter, r *http.Request) {
	columnID, err := utils.GetIdFromPath(r, "columnID")
	if err != nil {
		s.respondWithError(r.Context(), w, apierrors.FromErr(http.StatusBadRequest, err))
		return
	}
	err = s.store.DeleteColumn(r.Context(), columnID)
	if err != nil {
		s.respondWithError(r.Context(), w, apierrors.FromDBErr(err))
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
	board, err := s.store.GetBoardByID(r.Context(), param.BoardID)
	if err != nil {
		s.respondWithError(r.Context(), w, apierrors.FromDBErr(err))
		return
	}
	dbColumns, err := s.store.GetColumns(r.Context(), board.ID)
	if err != nil {
		s.respondWithError(r.Context(), w, apierrors.FromDBErr(err))
		return
	}
	s.respondWithJSON(w, 200, dbToColumnSlice(dbColumns))
}

func positionColumn(context context.Context, q *database.Queries, boardColumns []database.Column, column database.Column) error {
	oldPosition := slices.IndexFunc(boardColumns, func(c database.Column) bool {
		return c.ID == column.ID
	})
	stopIdx := len(boardColumns)
	if oldPosition != -1 {
		stopIdx = oldPosition
	}
	var err error
	if oldPosition < int(column.Position) {
		for i := int(column.Position); i > stopIdx; i-- {
			column := boardColumns[i]
			column.Position--
			err = q.UpdateColumnPosition(context, database.UpdateColumnPositionParams{
				ID:       column.ID,
				Position: column.Position,
			})
			if err != nil {
				break
			}
		}
	} else {
		for i := int(column.Position); i < stopIdx; i++ {
			column := boardColumns[i]
			column.Position++
			err = q.UpdateColumnPosition(context, database.UpdateColumnPositionParams{
				ID:       column.ID,
				Position: column.Position,
			})
			if err != nil {
				break
			}
		}
	}
	return err
}

func (s *server) handlerCreateColumn(w http.ResponseWriter, r *http.Request) {
	params, err := decodeJSONBody[columns.CreateParams](r)
	if err != nil {
		s.respondWithError(r.Context(), w, malformedBodyErr)
		return
	}
	_, err = s.store.GetBoardByID(r.Context(), params.BoardID)
	if err != nil {
		s.respondWithError(r.Context(), w, apierrors.FromDBErr(err))
		return
	}
	existingColumns, err := s.store.GetColumns(r.Context(), params.BoardID)
	if err != nil {
		s.respondWithError(r.Context(), w, apierrors.FromDBErr(err))
		return
	}
	if err := validateColumn(params, len(existingColumns)); err != nil {
		s.respondWithError(r.Context(), w, apierrors.FromErr(http.StatusBadRequest, err))
		return
	}
	var dbColumn database.Column
	err = s.store.execTx(r.Context(), func(qtx *database.Queries) error {
		var err error
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
		err = positionColumn(r.Context(), qtx, existingColumns, dbColumn)
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		s.respondWithError(r.Context(), w, err)
		return
	}
	s.respondWithJSON(w, http.StatusCreated, dbToColumn(dbColumn))
}

func dbToColumnSlice(dbColumns []database.Column) []columns.Column {
	columns := []columns.Column{}
	for _, dbColumn := range dbColumns {
		columns = append(columns, dbToColumn(dbColumn))
	}
	return columns
}

func dbToColumn(dbColumn database.Column) columns.Column {
	return columns.Column{
		ID:          dbColumn.ID,
		Title:       dbColumn.Title,
		CreatedAt:   dbColumn.CreatedAt,
		UpdatedAt:   dbColumn.UpdatedAt,
		Description: dbColumn.Description.String,
		BoardID:     dbColumn.BoardID,
		Position:    int(dbColumn.Position),
	}
}

func validateColumn(params columns.CreateParams, existingColumnsCount int) error {
	var err []error
	if params.Position < 0 || params.Position > utils.IntToInt32(existingColumnsCount) {
		err = append(err, fmt.Errorf("body.position outside correct range [0, %d]", existingColumnsCount))
	}
	if params.BoardID == uuid.Nil {
		err = append(err, errors.New("body.board_id is required"))
	}
	if params.Title == "" {
		err = append(err, errors.New("body.title is required"))
	}
	return errors.Join(err...)
}
