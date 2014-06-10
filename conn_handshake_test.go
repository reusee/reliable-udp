package udp

import (
	"fmt"
	"math/rand"
	"testing"
)

func TestHandshake(t *testing.T) {
	addr := fmt.Sprintf("127.0.0.1:%d", rand.Intn(20000)+20000)

	server, err := NewServer(addr)
	if err != nil {
		t.Fatalf("NewServer")
	}
	defer server.Close()
	go func() { // server logs
		for log := range server.Logs {
			p("SERVER: %s\n", log)
		}
	}()

	waitServerDone := make(chan bool)
	go func() {
		for conn := range server.NewConns { // new connection
			go func() {
				for log := range conn.Logs {
					p("SERVER CONN: %s\n", log)
					if log == "handshake done" {
						close(waitServerDone)
					}
				}
			}()
		}
	}()

	client, err := NewClient(addr)
	if err != nil {
		t.Fatalf("NewClient")
	}
	defer client.Close()

	waitClientDone := make(chan bool)
	go func() {
		for log := range client.Logs {
			p("CLIENT CONN: %s\n", log)
			if log == "handshake done" {
				close(waitClientDone)
			}
		}
	}()

	<-waitClientDone
	<-waitServerDone
}

func TestHandshakeTimeout(t *testing.T) {
	_, err := NewClient("12.34.56.78:9012")
	if err == nil {
		t.Fatalf("should fail")
	}
	if err.Error() != "handshake timeout" {
		t.Fatalf("should be handshake timeout error")
	}
}
