package columns

import (
	"time"

	"github.com/google/uuid"
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
