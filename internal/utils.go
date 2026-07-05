package utils

import (
	"net/http"

	"github.com/google/uuid"
)

func GetIdFromPath(r *http.Request, value string) (uuid.UUID, error) {
	idQuery := r.PathValue(value)
	id, err := uuid.Parse(idQuery)
	return id, err
}
