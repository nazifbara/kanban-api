package main

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"slices"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/nazifbara/kanban-api/internal/database"
)

type Task struct {
	ID          uuid.UUID `json:"id"`
	BoardID     uuid.UUID `json:"board_id"`
	ColumnID    uuid.UUID `json:"column_id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Position    int       `json:"position"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type CreateTaskParam struct {
	ColumnID    uuid.UUID `json:"column_id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Position    int       `json:"position"`
}

func validateCreateTaskParam(param CreateTaskParam) error {
	var err []error
	if param.ColumnID == uuid.Nil {
		err = append(err, errors.New("body.column_id is required"))
	}
	if param.Title == "" {
		err = append(err, errors.New("body.title is required"))
	}
	if utf8.RuneCountInString(param.Title) > 255 {
		err = append(err, errors.New("body.title cannot excedd 255 characters"))
	}
	return errors.Join(err...)
}

func (s *server) handlerCreateTask(w http.ResponseWriter, r *http.Request) {
	params, err := decodeJSONBody[CreateTaskParam](r)
	if err != nil {
		respondWithError(r.Context(), w, http.StatusBadRequest, malformedBodyErr)
		return
	}
	if err := validateCreateTaskParam(params); err != nil {
		respondWithError(r.Context(), w, http.StatusBadRequest, err)
		return
	}
	dbColumn, err := s.store.GetColumnById(r.Context(), params.ColumnID)
	if err != nil {
		respondFromDBErr(r.Context(), w, err)
		return
	}
	existingTasks, err := s.store.GetColumnTasks(r.Context(), database.GetColumnTasksParams{
		ColumnID: params.ColumnID,
		BoardID:  dbColumn.BoardID,
	})
	if err != nil {
		respondFromDBErr(r.Context(), w, err)
		return
	}
	var dbTask database.Task
	s.store.execTx(r.Context(), func(qtx *database.Queries) error {
		dbTask, err = qtx.CreateTask(
			r.Context(),
			database.CreateTaskParams{
				BoardID:     dbColumn.BoardID,
				ColumnID:    dbColumn.ID,
				Description: sql.NullString{String: params.Description, Valid: true},
				Title:       params.Title,
				Position:    int32(params.Position),
			},
		)
		if err != nil {
			return err
		}
		if err := positionTask(r.Context(), qtx, existingTasks, dbTask); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		respondWith500(r.Context(), w, err)
		return
	}
	respondWithJSON(w, http.StatusCreated, dbToTask(dbTask))
}

func positionTask(context context.Context, q *database.Queries, columnTasks []database.Task, dbTask database.Task) error {
	oldPosition := slices.IndexFunc(columnTasks, func(t database.Task) bool {
		return t.ID == dbTask.ID
	})
	stopIdx := len(columnTasks)
	if oldPosition != -1 {
		stopIdx = oldPosition
	}
	var err error
	if oldPosition < int(dbTask.Position) {
		for i := int(dbTask.Position); i > stopIdx; i-- {
			task := columnTasks[i]
			task.Position--
			err = q.UpdateTaskPosition(context, database.UpdateTaskPositionParams{
				ID:       task.ID,
				Position: task.Position,
			})
			if err != nil {
				break
			}
		}
	} else {
		for i := int(dbTask.Position); i < stopIdx; i++ {
			task := columnTasks[i]
			task.Position++
			err = q.UpdateTaskPosition(context, database.UpdateTaskPositionParams{
				ID:       task.ID,
				Position: task.Position,
			})
			if err != nil {
				break
			}
		}
	}
	return err
}

func dbToTask(dbTask database.Task) Task {
	return Task{
		ID:          dbTask.ID,
		BoardID:     dbTask.BoardID,
		ColumnID:    dbTask.ColumnID,
		Title:       dbTask.Title,
		Description: dbTask.Description.String,
		CreatedAt:   dbTask.CreatedAt,
		UpdatedAt:   dbTask.UpdatedAt,
	}
}
