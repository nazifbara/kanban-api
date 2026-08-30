package utils

import (
	"fmt"
	"math"
	"net/http"
	"regexp"

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

// IsValidEmail checks if the string is a valid email address.
func IsValidEmail(email string) bool {
	// This regex ensures:
	// 1. Alphanumeric/allowed symbols before the @
	// 2. An @ symbol
	// 3. A domain name with at least one dot (.)
	// 4. A TLD (top-level domain) of at least 2 characters
	emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)

	return emailRegex.MatchString(email)
}

func GetIdFromPath(r *http.Request, name string) (uuid.UUID, error) {
	idQuery := r.PathValue(name)
	id, err := uuid.Parse(idQuery)
	if err != nil {
		err = fmt.Errorf("invalid ID value value for named value %s", name)
	}
	return id, err
}
