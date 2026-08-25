package columns

import (
	"time"

	"github.com/google/uuid"
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

type CreateParams struct {
	Title       string    `json:"title"`
	BoardID     uuid.UUID `json:"board_id"`
	Position    int32     `json:"position"`
	Description string    `json:"description"`
}
type PatchParams struct {
	Title       *string `json:"title"`
	Description *string `json:"description"`
	Position    *int32  `json:"position"`
}

type ColumnBoardID struct {
	BoardID uuid.UUID `json:"board_id"`
}

type PositionsParams struct {
	BoardID   uuid.UUID   `json:"board_id"`
	Positions []uuid.UUID `json:"positions"`
}

func DBToColumnSlice(dbColumns []database.Column) []Column {
	columns := []Column{}
	for _, dbColumn := range dbColumns {
		columns = append(columns, DBToColumn(dbColumn))
	}
	return columns
}

func DBToColumn(dbColumn database.Column) Column {
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
