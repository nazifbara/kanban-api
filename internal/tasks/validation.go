package tasks

import (
	"database/sql"
	"errors"
	"fmt"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/nazifbara/kanban-api/internal/database"
	"github.com/nazifbara/kanban-api/internal/utils"
)

func ValidatePatchParam(params PatchParam, maxPosition *int) error {
	var errs []error
	if params.Position != nil && maxPosition != nil && (*params.Position < 0 || *params.Position > utils.IntToInt32(*maxPosition)) {
		errs = append(errs, fmt.Errorf("body.position out of range [0, %d]", maxPosition))
	}
	if params.ColumnID != nil && *params.ColumnID == uuid.Nil {
		errs = append(errs, errors.New("body.column_id is required"))
	}
	if params.Title != nil && *params.Title == "" {
		errs = append(errs, errors.New("body.title is required"))
	}
	if params.Title != nil && utf8.RuneCountInString(*params.Title) > 255 {
		errs = append(errs, errors.New("body.title cannot exceed 255 characters"))
	}
	return errors.Join(errs...)
}

func ValidateCreateParam(params CreateParam, maxPosition int) error {
	var errs []error
	if params.Position < 0 || params.Position > utils.IntToInt32(maxPosition) {
		errs = append(errs, fmt.Errorf("body.position out of range [0, %d]", maxPosition))
	}
	if params.Title == "" {
		errs = append(errs, errors.New("body.title is required"))
	}
	if utf8.RuneCountInString(params.Title) > 255 {
		errs = append(errs, errors.New("body.title cannot exceed 255 characters"))
	}
	return errors.Join(errs...)
}

func PrepareTaskPatch(param PatchParam) database.UpdateTaskParams {
	var patch database.UpdateTaskParams
	if param.ColumnID != nil {
		patch.ColumnID = uuid.NullUUID{UUID: *param.ColumnID, Valid: true}
	}
	if param.Title != nil {
		patch.Title = sql.NullString{String: *param.Title, Valid: true}
	}
	if param.Description != nil {
		patch.Description = sql.NullString{String: *param.Description, Valid: true}
	}
	if param.Position != nil {
		patch.Position = sql.NullInt32{Int32: *param.Position, Valid: true}
	}
	return patch
}
