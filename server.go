package udp

import (
	"log"
	"net"

	"github.com/reusee/closer"
	ic "github.com/reusee/inf-chan"
)

type Server struct {
	closer.Closer
	*Logger

	udpConn       *net.UDPConn
	udpConnClosed bool

	newConnsIn chan *Conn
	NewConns   chan *Conn
}

func NewServer(addrStr string) (*Server, error) {
	addr, err := net.ResolveUDPAddr("udp", addrStr)
	if err != nil {
		return nil, err
	}
	udpConn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return nil, err
	}
	server := &Server{
		Logger:     newLogger(),
		udpConn:    udpConn,
		newConnsIn: make(chan *Conn),
		NewConns:   make(chan *Conn),
	}
	ic.Link(server.newConnsIn, server.NewConns)
	server.OnClose(func() {
		server.udpConnClosed = true
		udpConn.Close()
		close(server.newConnsIn)
	})

	go func() { // listen
		conns := make(map[string]*Conn)
		for {
			// read packet
			packetData := make([]byte, 1500)
			n, addr, err := udpConn.ReadFromUDP(packetData)
			if err != nil {
				if server.udpConnClosed {
					return
				} else {
					log.Fatalf("server read error %v", err)
				}
			}
			packetData = packetData[:n]
			server.Log("ReadFromUDP length %d", n)

			// dispatch
			key := addr.String()
			conn, ok := conns[key]
			if !ok { // new connection
				server.newConn(conns, addr, packetData)
			} else {
				conn.incomingPacketsIn <- packetData
			}
		}
	}()

	return server, nil
}

func (s *Server) readPacket(packetData []byte) (uint32, uint32, byte, uint16, []byte) {
	serial, ackSerial, flag, windowSize, data := readPacket(packetData)
	s.Log("readPacket serial %d ackSerial %d flag %x windowSize %d length %d",
		serial, ackSerial, flag, windowSize, len(data))
	return serial, ackSerial, flag, windowSize, data
}

func (s *Server) newConn(conns map[string]*Conn, remoteAddr *net.UDPAddr, packetData []byte) {
	serial, _, flag, _, _ := s.readPacket(packetData)
	if flag&SYNC == 0 { // not a sync packet
		s.Log("newConn ignore from %s", remoteAddr.String())
		return
	}
	conn := makeConn()
	conn.writeUDP = func(data []byte) error {
		_, err := s.udpConn.WriteToUDP(data, remoteAddr)
		return err
	}
	conn.ackSerial = serial + 1
	conns[remoteAddr.String()] = conn
	conn.OnClose(func() {
		delete(conns, remoteAddr.String())
	})
	s.newConnsIn <- conn
	go conn.handshake()
	s.Log("newConn addr %s serial %d", remoteAddr.String(), serial)
}
