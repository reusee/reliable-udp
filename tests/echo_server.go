package main

import (
	"fmt"
	"log"

	udp "../"
)

func main() {
	server, err := udp.NewServer("0.0.0.0:50000")
	if err != nil {
		log.Fatal(err)
	}
	go func() {
		for log := range server.Logs {
			fmt.Printf("SERVER: %s\n", log)
		}
	}()
	for conn := range server.NewConns {
		go func() {
			go func() {
				for log := range conn.Logs {
					fmt.Printf("CONN: %s\n", log)
				}
				for data := range conn.Recv {
					conn.Send(data)
				}
			}()
		}()
	}
}
