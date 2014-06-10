package udp

import (
	"fmt"
	"math/rand"
	"time"
)

var p = fmt.Printf

func init() {
	rand.Seed(time.Now().UnixNano())
}
