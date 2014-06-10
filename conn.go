package udp

import (
	"bytes"
	"container/heap"
	"container/list"
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
	recvIn            chan []byte
	Recv              chan []byte

	unackPackets *list.List
	packetHeap   *Heap
}

func makeConn() *Conn {
	conn := &Conn{
		serial:            uint32(rand.Intn(65536)),
		Logger:            newLogger(),
		incomingPacketsIn: make(chan []byte),
		incomingPackets:   make(chan []byte),
		recvIn:            make(chan []byte),
		Recv:              make(chan []byte),
		unackPackets:      list.New(),
		packetHeap:        new(Heap),
	}
	heap.Init(conn.packetHeap)
	ic.Link(conn.incomingPacketsIn, conn.incomingPackets)
	ic.Link(conn.recvIn, conn.Recv)
	conn.OnClose(func() {
		conn.Logger.Close()
		close(conn.incomingPacketsIn)
		close(conn.recvIn)
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

func (c *Conn) start() {
	for {
		select {
		case packetData, ok := <-c.incomingPackets:
			if !ok { // conn closed
				return
			}
			c.handlePacket(packetData)
		}
	}
}

func (c *Conn) handlePacket(packetData []byte) {
	serial, ackSerial, flags, windowSize, data := c.readPacket(packetData)
	if serial == c.ackSerial { // in order
		if len(data) > 0 {
			c.recvIn <- data
			c.Log("Provide serial %d length %d", serial, len(data))
		}
		c.ackSerial++
	} else if serial < c.ackSerial { // duplicated packet
		// ignore
	} else if serial > c.ackSerial { // out of order
		heap.Push(c.packetHeap, &Packet{ // push to heap
			serial: serial,
			data:   data,
		})
		c.Log("Out of order packet %d heapLen %d", serial, c.packetHeap.Len())
		for packet := c.packetHeap.Peek(); packet.serial == c.ackSerial; packet = c.packetHeap.Peek() {
			heap.Pop(c.packetHeap)
			if len(packet.data) > 0 {
				c.recvIn <- packet.data
				c.Log("Provide serial %d length %d", serial, len(data))
			}
			c.ackSerial++
		}
	}
	// process ackSerial
	c.Log("unackPackets Len %d", c.unackPackets.Len())
	for e := c.unackPackets.Front(); e != nil && e.Value.(Packet).serial < ackSerial; e = c.unackPackets.Front() {
		c.Log("Acked %d", e.Value.(Packet).serial)
		c.unackPackets.Remove(e)
	}
	_ = ackSerial
	//TODO process flags
	_ = flags
	//TODO windowSize
	_ = windowSize
}

func (c *Conn) Send(data []byte) error {
	packet := c.newPacket(data, ACK)
	c.unackPackets.PushBack(packet)
	c.Log("Send serial %d", packet.serial)
	return c.sendPacket(packet)
}
