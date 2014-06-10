package udp

import (
	"bytes"
	"encoding/binary"
	"errors"
	"math/rand"
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
		serial:            uint32(rand.Intn(65536)),
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
	c.Log("readPacket serial %d ackSerial %d flag %b windowSize %d length %d",
		serial, ackSerial, flag, windowSize, len(data))
	return serial, ackSerial, flag, windowSize, data
}

func (c *Conn) handshake() error {
	c.Log("handshake start")
	// send sync packet
	packet := c.newPacket([]byte{}, SYNC, ACK)
	c.sendPacket(packet)
	// wait ack
	select {
	case <-time.NewTimer(time.Second * 4).C: // timeout
		c.Log("handshake timeout")
		c.Close()
		return errors.New("handshake timeout")
	case packetData := <-c.incomingPackets: // get ack
		serial, _, flags, _, _ := c.readPacket(packetData)
		if flags&ACK == 0 {
			c.Log("handshake error")
			c.Close()
			return errors.New("handshake error")
		}
		c.ackSerial = serial + 1
	}
	c.Log("handshake done")
	return nil
}
