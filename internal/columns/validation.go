package columns

import (
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/nazifbara/kanban-api/internal/utils"
)

func ValidatePatch(patch PatchParams, existingColumnsCount int) error {
	var errs []error
	if patch.Position != nil && *patch.Position < 0 || *patch.Position > utils.IntToInt32(existingColumnsCount) {
		errs = append(errs, fmt.Errorf("body.position outside correct range [0, %d]", existingColumnsCount))
	}
	if patch.Title != nil && *patch.Title == "" {
		errs = append(errs, errors.New("body.title is required"))
	}
	return errors.Join(errs...)
}

func ValidateColumn(params CreateParams, existingColumnsCount int) error {
	var err []error
	if params.Position < 0 || params.Position > utils.IntToInt32(existingColumnsCount) {
		err = append(err, fmt.Errorf("body.position outside correct range [0, %d]", existingColumnsCount))
	}

	if params.Title == "" {
		err = append(err, errors.New("body.title is required"))
	}
	return errors.Join(err...)
}

func ValidatePositionsParams(params PositionsParams) error {
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
