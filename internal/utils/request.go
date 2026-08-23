package utils

import (
	"fmt"
	"net/http"

	"github.com/google/uuid"
)

func GetIdFromPath(r *http.Request, name string) (uuid.UUID, error) {
	idQuery := r.PathValue(name)
	id, err := uuid.Parse(idQuery)
	if err != nil {
		err = fmt.Errorf("invalid ID value value for named value %s", name)
	}
	return id, err
}
