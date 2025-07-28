package random

import (
	"math/rand"
	"strings"
	"time"
)

func GetRandom(length int) string {
	rand.Seed(time.Now().UnixNano())
	var sb strings.Builder
	sb.Grow(length)
	for i := 0; i < length; i++ {
		sb.WriteByte(byte(rand.Intn(10) + '0'))
	}
	return sb.String()
}
