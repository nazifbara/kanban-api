package tasks

import (
	"database/sql"
	"errors"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/nazifbara/kanban-api/internal/database"
)

func ValidateCreateParam(param CreateParam) error {
	var errs []error
	if param.ColumnID == uuid.Nil {
		errs = append(errs, errors.New("body.column_id is required"))
	}
	if param.Title == "" {
		errs = append(errs, errors.New("body.title is required"))
	}
	if utf8.RuneCountInString(param.Title) > 255 {
		errs = append(errs, errors.New("body.title cannot exceed 255 characters"))
	}
	return errors.Join(errs...)
}
func PrepareTaskPatch(param UpdateParam) (database.UpdateTaskParams, error) {
	var patch database.UpdateTaskParams
	if param.ColumnID != nil {
		patch.ColumnID = uuid.NullUUID{UUID: *param.ColumnID, Valid: true}
	}
	if param.Title != nil {
		if *param.Title == "" {
			return patch, errors.New("body.title can't be empty")
		}
		patch.Title = sql.NullString{String: *param.Title, Valid: true}
	}
	if param.Description != nil {
		patch.Description = sql.NullString{String: *param.Description, Valid: true}
	}
	if param.Position != nil {
		patch.Position = sql.NullInt32{Int32: *param.Position, Valid: true}
	}
	return patch, nil
}
