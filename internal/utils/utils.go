package utils

import (
	"fmt"
	"math"
	"net/http"

	"github.com/google/uuid"
)

func Ptr[T any](v T) *T {
	res := new(T)
	*res = v
	return res
}

func IntToInt32(n int) int32 {
	if n > math.MaxInt32 || n < math.MinInt32 {
		panic(fmt.Sprintf("utils: value %d overflows int32", n))
	}
	return int32(n)
}

func GetIdFromPath(r *http.Request, name string) (uuid.UUID, error) {
	idQuery := r.PathValue(name)
	id, err := uuid.Parse(idQuery)
	if err != nil {
		err = fmt.Errorf("invalid ID value value for named value %s", name)
	}
	return id, err
}
