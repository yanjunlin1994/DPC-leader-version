package wendy

import (
	"errors"
	"fmt"
)

const (
	LogLevelDebug = iota
	LogLevelWarn
	LogLevelError
)




// job for each hashcat instance
type job struct {
	HashType  int // Type of hash
	HashValue string // Hash value
	Length    int
	Start     int
	Limit     int
}
