package util

import "math/rand"

func New(seed int64) *rand.Rand {
	if seed == 0 {
		seed = 1
	}
	src := rand.NewSource(seed)
	return rand.New(src)
}
