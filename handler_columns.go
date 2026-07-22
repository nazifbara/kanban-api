package main

import (
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
}

type ColumnParams struct {
	Title   string    `json:"title"`
	BoardID uuid.UUID `json:"board_id"`
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

func (s *server) handlerCreateColumn(w http.ResponseWriter, r *http.Request) {
	params, err := decodeJSONBody[ColumnParams](r)
	if err != nil {
		respondWithError(r.Context(), w, http.StatusBadRequest, fmt.Errorf("malformed request body"))
		return
	}
	if err := validateColumn(params); err != nil {
		respondWithError(r.Context(), w, http.StatusBadRequest, err)
		return
	}
	var dbColumn database.Column
	s.store.execTx(r.Context(), func(qtx *database.Queries) error {
		dbColumn, err = qtx.CreateColumn(
			r.Context(),
			database.CreateColumnParams{BoardID: params.BoardID, Title: params.Title},
		)
		if err != nil {
			return err
		}
		dbBoard, err := qtx.GetBoardByID(r.Context(), params.BoardID)
		if err != nil {
			return err
		}
		dbBoard.ColumnPositions = append(dbBoard.ColumnPositions, dbColumn.ID)
		_, err = qtx.AdjustBoardPositions(r.Context(), database.AdjustBoardPositionsParams{
			ID:              params.BoardID,
			ColumnPositions: dbBoard.ColumnPositions,
		})
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
	}
}

func validateColumn(params ColumnParams) error {
	if params.BoardID == uuid.Nil {
		return errors.New("body.board_id is required")
	}
	if params.Title == "" {
		return errors.New("body.title is required")
	}
	return nil
}
