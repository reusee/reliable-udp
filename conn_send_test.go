package udp

import (
	"fmt"
	"math/rand"
	"testing"
)

func TestSend(t *testing.T) {
	addr := fmt.Sprintf("127.0.0.1:%d", rand.Intn(20000)+20000)

	server, err := NewServer(addr)
	if err != nil {
		t.Fatalf("NewServer")
	}
	defer server.Close()
	go func() { //server logs
		for log := range server.Logs {
			p("SERVER: %s\n", log)
		}
	}()

	n := 102400
	done := make(chan bool)

	go func() {
		for conn := range server.NewConns {
			go func() { // show logs
				for log := range conn.Logs {
					p("SERVER CONN: %s\n", log)
				}
			}()
			for i := 0; i < n; i++ {
				data := <-conn.Recv
				p("SERVER CONN RECEIVE: %s\n", data)
			}
			close(done)
		}
	}()

	client, err := NewClient(addr)
	if err != nil {
		t.Fatalf("NewClient")
	}
	//defer client.Close() TODO fix closing bug

	go func() {
		for log := range client.Logs {
			p("CLIENT CONN: %s\n", log)
		}
	}()

	for i := 0; i < n; i++ {
		client.Send([]byte(fmt.Sprintf("%d", i)))
	}

	<-done

	p("resend %d\n", client.StatResend)
}
