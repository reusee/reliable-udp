package udp

import (
	"bytes"
	"encoding/binary"
	"time"

	"github.com/reusee/closer"
	ic "github.com/reusee/inf-chan"
)

type Conn struct {
	closer.Closer
	*Logger

	writeUDP   func([]byte) error
	serial     uint32
	ackSerial  uint32
	windowSize uint16

	incomingPacketsIn chan []byte
	incomingPackets   chan []byte
}

func makeConn() *Conn {
	conn := &Conn{
		Logger:            newLogger(),
		incomingPacketsIn: make(chan []byte),
		incomingPackets:   make(chan []byte),
	}
	ic.Link(conn.incomingPacketsIn, conn.incomingPackets)
	conn.OnClose(func() {
		conn.Logger.Close()
		close(conn.incomingPacketsIn)
	})
	return conn
}

func (c *Conn) sendPacket(packet Packet) error {
	buf := new(bytes.Buffer)
	binary.Write(buf, binary.LittleEndian, packet.serial)
	binary.Write(buf, binary.LittleEndian, c.ackSerial)
	buf.WriteByte(packet.flag)
	binary.Write(buf, binary.LittleEndian, c.windowSize)
	buf.Write(packet.data)
	c.Log("sendPacket serial %d ack %d window %d", packet.serial, c.ackSerial, c.windowSize)
	return c.writeUDP(buf.Bytes())
}

func (c *Conn) readPacket(packetData []byte) (uint32, uint32, byte, uint16, []byte) {
	serial, ackSerial, flag, windowSize, data := readPacket(packetData)
	c.Log("readPacket serial %d ackSerial %d flag %x windowSize %d length %d",
		serial, ackSerial, flag, windowSize, len(data))
	return serial, ackSerial, flag, windowSize, data
}

func (c *Conn) handshake() {
	c.Log("handshake start")
	// send sync packet
	packet := c.newPacket([]byte{}, SYNC, ACK)
	c.sendPacket(packet)
	// wait ack
	select {
	case <-time.NewTimer(time.Second * 10).C: // timeout
		c.Log("handshake timeout")
		c.Close()
		return
	case packetData := <-c.incomingPackets:
		serial, _, _, _, _ := c.readPacket(packetData)
		c.ackSerial = serial + 1
	}
	c.Log("handshake done")
}
