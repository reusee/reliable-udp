package udp

import (
	"fmt"
	"math/rand"
	"testing"
)

func TestServerNew(t *testing.T) {
	addr := fmt.Sprintf("127.0.0.1:%d", rand.Intn(20000)+20000)
	server, err := NewServer(addr)
	if err != nil {
		t.Fail()
	}
	server.Close()
}
