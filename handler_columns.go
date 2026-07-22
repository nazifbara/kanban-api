package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	utils "github.com/nazifbara/kanban-api/internal"
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
	Title    string    `json:"title"`
	BoardID  uuid.UUID `json:"board_id"`
	Position int       `json:"position"`
}

type columnBoardID struct {
	BoardID uuid.UUID `json:"board_id"`
}

func (s *server) handlerDeleteColumn(w http.ResponseWriter, r *http.Request) {
	columnID, err := utils.GetIdFromPath(r, "columnID")
	if err != nil {
		respondWithError(r.Context(), w, http.StatusBadRequest, fmt.Errorf("invalid board"))
		return
	}
	err = s.store.DeleteColumn(r.Context(), columnID)
	if err != nil {
		respondWith500(r.Context(), w, err)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (s *server) handlerBoardColumns(w http.ResponseWriter, r *http.Request) {
	param, err := decodeJSONBody[columnBoardID](r)
	if err != nil {
		respondWithError(r.Context(), w, http.StatusBadRequest, fmt.Errorf("Invalid request body"))
		return
	}
	board, err := s.store.GetBoardByID(r.Context(), param.BoardID)
	if err != nil {
		respondFromDBErr(r.Context(), w, err)
		return
	}
	dbColumns, err := s.store.GetColumns(r.Context(), board.ID)
	if err != nil {
		respondFromDBErr(r.Context(), w, err)
		return
	}
	respondWithJSON(w, 200, dbToColumnSlice(dbColumns))
}

func handleColumnShifts(context context.Context, q *database.Queries, existingColumns []database.Column, newPosition int) error {
	var err error
	for i := newPosition; i < len(existingColumns); i++ {
		column := existingColumns[i]
		column.Position++
		err = q.UpdateColumnPosition(context, database.UpdateColumnPositionParams{
			ID:       column.ID,
			Position: column.Position,
		})
		if err != nil {
			break
		}
	}
	return err
}

func (s *server) handlerCreateColumn(w http.ResponseWriter, r *http.Request) {
	params, err := decodeJSONBody[ColumnParams](r)
	if err != nil {
		respondWithError(r.Context(), w, http.StatusBadRequest, fmt.Errorf("malformed request body"))
		return
	}
	_, err = s.store.GetBoardByID(r.Context(), params.BoardID)
	if err != nil {
		respondFromDBErr(r.Context(), w, err)
		return
	}
	existingColumns, err := s.store.GetColumns(r.Context(), params.BoardID)
	if err != nil {
		respondFromDBErr(r.Context(), w, err)
		return
	}
	if err := validateColumn(params, len(existingColumns)); err != nil {
		respondWithError(r.Context(), w, http.StatusBadRequest, err)
		return
	}
	var dbColumn database.Column
	s.store.execTx(r.Context(), func(qtx *database.Queries) error {
		err = handleColumnShifts(r.Context(), qtx, existingColumns, params.Position)
		if err != nil {
			return err
		}
		dbColumn, err = qtx.CreateColumn(
			r.Context(),
			database.CreateColumnParams{
				BoardID:  params.BoardID,
				Title:    params.Title,
				Position: int32(params.Position),
			},
		)
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		respondWith500(r.Context(), w, err)
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
