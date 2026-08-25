package utils

import (
	"fmt"
	"math"
)

func IntToInt32(n int) int32 {
	if n > math.MaxInt32 || n < math.MinInt32 {
		panic(fmt.Sprintf("utils: value %d overflows int32", n))
	}
	return int32(n)
}
