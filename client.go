package udp

import (
	"log"
	"net"
)

func NewClient(addrStr string) (*Conn, error) {
	addr, err := net.ResolveUDPAddr("udp", addrStr)
	if err != nil {
		return nil, err
	}
	udpConn, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		return nil, err
	}
	conn := makeConn()
	conn.writeUDP = func(data []byte) error {
		_, err := udpConn.Write(data)
		return err
	}
	udpConnClosed := false
	conn.OnClose(func() {
		udpConnClosed = true
		udpConn.Close()
	})

	go func() {
		for {
			packetData := make([]byte, 1500)
			n, _, err := udpConn.ReadFromUDP(packetData)
			if err != nil {
				if udpConnClosed {
					return
				} else {
					log.Fatalf("client read error %v", err)
				}
			}
			packetData = packetData[:n]
			conn.Log("ReadFromUDP length %d", n)
			conn.incomingPacketsIn <- packetData
		}
	}()

	// handshake
	if err := conn.handshake(); err != nil {
		return nil, err
	}
	syncPacket := conn.newPacket([]byte{}, ACK)
	conn.sendPacket(syncPacket)
	go conn.start()

	return conn, nil
}
