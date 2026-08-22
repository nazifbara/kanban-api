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
	utils "github.com/nazifbara/kanban-api/internal"
	"github.com/nazifbara/kanban-api/internal/apierrors"
	"github.com/nazifbara/kanban-api/internal/database"
)

type Column struct {
	ID          uuid.UUID `json:"id"`
	Title       string    `json:"title"`
	BoardID     uuid.UUID `json:"board_id"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	Description string    `json:"description"`
	Position    int       `json:"position"`
}

type ColumnParams struct {
	Title       string    `json:"title"`
	BoardID     uuid.UUID `json:"board_id"`
	Position    int       `json:"position"`
	Description string    `json:"description"`
}
type PatchColumnParams struct {
	Title       *string `json:"title"`
	Description *string `json:"description"`
	Position    *int    `json:"position"`
}

type columnBoardID struct {
	BoardID uuid.UUID `json:"board_id"`
}

func prepareColumnPatch(params PatchColumnParams) database.UpdateColumnParams {
	var patch database.UpdateColumnParams
	if params.Title != nil {
		patch.Title = sql.NullString{String: *params.Title, Valid: true}
	}
	if params.Description != nil {
		patch.Description = sql.NullString{String: *params.Description, Valid: true}
	}
	if params.Position != nil {
		patch.Position = sql.NullInt32{Int32: int32(*params.Position), Valid: true}
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
		respondWithError(r.Context(), w, malformedBodyErr)
		return
	}
	if err := validateColumnPositionsParams(params); err != nil {
		respondWithError(r.Context(), w, apierrors.FromErr(http.StatusBadRequest, err))
		return
	}
	_, err = s.store.GetBoardByID(r.Context(), params.BoardID)
	if err != nil {
		respondWithError(r.Context(), w, apierrors.FromDBErr(err))
		return
	}
	boardColumns, err := s.store.GetColumns(r.Context(), params.BoardID)
	if err != nil {
		respondWithError(r.Context(), w, apierrors.FromDBErr(err))
		return
	}
	if len(boardColumns) != len(params.Positions) {
		respondWithError(r.Context(), w, apierrors.New(http.StatusBadRequest, "positions don't match board columns count"))
		return
	}

	for _, columnID := range params.Positions {
		if !slices.ContainsFunc(boardColumns, func(c database.Column) bool { return c.ID == columnID }) {
			respondWithError(r.Context(), w, apierrors.New(http.StatusBadRequest, "at least one column doesn't belong to the board"))
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
		respondWithError(r.Context(), w, err)
		return
	}
	slices.SortFunc(boardColumns, func(a, b database.Column) int { return int(a.Position - b.Position) })
	respondWithJSON(w, http.StatusOK, dbToColumnSlice(boardColumns))
}

func (s *server) handlerPatchColumn(w http.ResponseWriter, r *http.Request) {
	columnID, err := utils.GetIdFromPath(r, "columnID")
	if err != nil {
		respondWithError(r.Context(), w, apierrors.FromErr(http.StatusBadRequest, err))
	}
	patchParams, err := decodeJSONBody[PatchColumnParams](r)
	if err != nil {
		respondWithError(r.Context(), w, malformedBodyErr)
		return
	}
	oldColumn, err := s.store.GetColumn(r.Context(), columnID)
	if err != nil {
		respondWithError(r.Context(), w, apierrors.FromDBErr(err))
		return
	}
	patch := prepareColumnPatch(patchParams)
	patch.ID = columnID
	boardColumns, err := s.store.GetColumns(r.Context(), oldColumn.BoardID)
	if patch.Position.Valid && (int(patch.Position.Int32) >= len(boardColumns) || patch.Position.Int32 < 0) {
		respondWithError(r.Context(), w, apierrors.New(http.StatusBadRequest, fmt.Sprintf("column position out of range [0, %d]", len(boardColumns))))
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
		respondWithError(r.Context(), w, err)
		return
	}
	respondWithJSON(w, http.StatusOK, dbToColumn(column))
}

func (s *server) handlerDeleteColumn(w http.ResponseWriter, r *http.Request) {
	columnID, err := utils.GetIdFromPath(r, "columnID")
	if err != nil {
		respondWithError(r.Context(), w, apierrors.FromErr(http.StatusBadRequest, err))
		return
	}
	err = s.store.DeleteColumn(r.Context(), columnID)
	if err != nil {
		respondWithError(r.Context(), w, apierrors.FromDBErr(err))
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (s *server) handlerBoardColumns(w http.ResponseWriter, r *http.Request) {
	param, err := decodeJSONBody[columnBoardID](r)
	if err != nil {
		respondWithError(r.Context(), w, malformedBodyErr)
		return
	}
	board, err := s.store.GetBoardByID(r.Context(), param.BoardID)
	if err != nil {
		respondWithError(r.Context(), w, apierrors.FromDBErr(err))
		return
	}
	dbColumns, err := s.store.GetColumns(r.Context(), board.ID)
	if err != nil {
		respondWithError(r.Context(), w, apierrors.FromDBErr(err))
		return
	}
	respondWithJSON(w, 200, dbToColumnSlice(dbColumns))
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
	params, err := decodeJSONBody[ColumnParams](r)
	if err != nil {
		respondWithError(r.Context(), w, malformedBodyErr)
		return
	}
	_, err = s.store.GetBoardByID(r.Context(), params.BoardID)
	if err != nil {
		respondWithError(r.Context(), w, apierrors.FromDBErr(err))
		return
	}
	existingColumns, err := s.store.GetColumns(r.Context(), params.BoardID)
	if err != nil {
		respondWithError(r.Context(), w, apierrors.FromDBErr(err))
		return
	}
	if err := validateColumn(params, len(existingColumns)); err != nil {
		respondWithError(r.Context(), w, apierrors.FromErr(http.StatusBadRequest, err))
		return
	}
	var dbColumn database.Column
	s.store.execTx(r.Context(), func(qtx *database.Queries) error {
		dbColumn, err = qtx.CreateColumn(
			r.Context(),
			database.CreateColumnParams{
				BoardID:  params.BoardID,
				Title:    params.Title,
				Position: int32(params.Position),
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
		respondWithError(r.Context(), w, err)
		return
	}
	respondWithJSON(w, http.StatusCreated, dbToColumn(dbColumn))
}

func dbToColumnSlice(dbColumns []database.Column) []Column {
	columns := []Column{}
	for _, dbColumn := range dbColumns {
		columns = append(columns, dbToColumn(dbColumn))
	}
	return columns
}

func dbToColumn(dbColumn database.Column) Column {
	return Column{
		ID:          dbColumn.ID,
		Title:       dbColumn.Title,
		CreatedAt:   dbColumn.CreatedAt,
		UpdatedAt:   dbColumn.UpdatedAt,
		Description: dbColumn.Description.String,
		BoardID:     dbColumn.BoardID,
		Position:    int(dbColumn.Position),
	}
}

func validateColumn(params ColumnParams, existingColumnsCount int) error {
	var err []error
	if params.Position < 0 || params.Position > existingColumnsCount {
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
