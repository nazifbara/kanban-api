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
type UpdateColumnParams struct {
	Title       string `json:"title"`
	Position    int    `json:"position"`
	Description string `json:"description"`
}

type columnBoardID struct {
	BoardID uuid.UUID `json:"board_id"`
}

func fillPatch(patchParams PatchColumnParams, oldColumn database.Column) (UpdateColumnParams, error) {
	var param UpdateColumnParams
	var err []error
	if patchParams.Title == nil {
		param.Title = oldColumn.Title
	} else {
		param.Title = *patchParams.Title
	}
	if patchParams.Description == nil {
		param.Description = oldColumn.Description.String
	} else {
		param.Description = *patchParams.Description
	}
	if patchParams.Position == nil {
		param.Position = int(oldColumn.Position)
	} else {
		param.Position = *patchParams.Position
	}
	return param, errors.Join(err...)
}

func (s *server) handlerPatchColumn(w http.ResponseWriter, r *http.Request) {
	columnID, err := utils.GetIdFromPath(r, "columnID")
	if err != nil {
		respondWithError(r.Context(), w, http.StatusBadRequest, errors.New("invalid column ID"))
	}
	patchParams, err := decodeJSONBody[PatchColumnParams](r)
	if err != nil {
		respondFromDBErr(r.Context(), w, err)
		return
	}
	oldColumn, err := s.store.GetColumnById(r.Context(), columnID)
	if err != nil {
		respondFromDBErr(r.Context(), w, err)
		return
	}
	param, err := fillPatch(patchParams, oldColumn)
	if err != nil {
		respondWithError(r.Context(), w, http.StatusBadRequest, err)
		return
	}
	boardColumns, err := s.store.GetColumns(r.Context(), oldColumn.BoardID)
	if param.Position >= len(boardColumns) || param.Position < 0 {
		respondWithError(r.Context(), w, http.StatusBadRequest, fmt.Errorf("column position out of range [0, %d]", len(boardColumns)))
		return
	}
	var column database.Column
	s.store.execTx(r.Context(), func(q *database.Queries) error {
		column, err = q.UpdateColumn(r.Context(), database.UpdateColumnParams{
			ID:          columnID,
			Description: sql.NullString{String: param.Description, Valid: true},
			Title:       param.Title,
			Position:    int32(param.Position),
		})
		if err != nil {
			return err
		}
		err = positionColumn(r.Context(), q, boardColumns, column)
		if err != nil {
			return err
		}
		return nil
	})
	respondWithJSON(w, http.StatusOK, dbToColumn(column))
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
		err = positionColumn(r.Context(), qtx, existingColumns, dbColumn)
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
