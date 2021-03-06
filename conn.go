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

var (
	ackTimerTimeout = time.Millisecond * 100
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

	unackPackets  *list.List
	packetHeap    *Heap
	ackCheckTimer *Timer
	ackTimer      *time.Timer

	StatResend int
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
		ackCheckTimer:     NewTimer(time.Millisecond * 100),
		ackTimer:          time.NewTimer(ackTimerTimeout),
	}
	heap.Init(conn.packetHeap)
	ic.Link(conn.incomingPacketsIn, conn.incomingPackets)
	ic.Link(conn.recvIn, conn.Recv)
	conn.OnClose(func() {
		conn.Logger.Close()
		close(conn.incomingPacketsIn)
		close(conn.recvIn)
		conn.ackCheckTimer.Close()
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
		case <-c.ackCheckTimer.Tick:
			c.checkAck()
		case <-c.ackTimer.C:
			c.sendAck()
		}
	}
}

func (c *Conn) handlePacket(packetData []byte) {
	// process data
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
		for c.packetHeap.Len() > 0 {
			packet := c.packetHeap.Peek()
			if packet.serial == c.ackSerial {
				heap.Pop(c.packetHeap)
				if len(packet.data) > 0 {
					c.recvIn <- packet.data
					c.Log("Provide serial %d length %d", serial, len(data))
				}
				c.ackSerial++
			} else if packet.serial < c.ackSerial { // duplicated
				heap.Pop(c.packetHeap)
			} else {
				break
			}
		}
	}
	// process ackSerial
	c.Log("unackPackets Len %d", c.unackPackets.Len())
	for e := c.unackPackets.Front(); e != nil; e = e.Next() {
		packet := e.Value.(*Packet)
		if packet.serial >= ackSerial {
			break
		}
		c.Log("Acked %d in %d", packet.serial, c.ackCheckTimer.Now-packet.sentTime)
		c.unackPackets.Remove(e)
	}
	//TODO process flags
	_ = flags
	//TODO windowSize
	_ = windowSize
}

func (c *Conn) checkAck() {
	now := c.ackCheckTimer.Now
	for e := c.unackPackets.Front(); e != nil; e = e.Next() { //TODO selective check
		packet := e.Value.(*Packet)
		if now > packet.sentTime && now-packet.sentTime > packet.resendTimeout { // check later
			c.Log("Timeout serial %d now %d sent %d timeout %d", packet.serial, now, packet.sentTime, packet.resendTimeout)
			c.sendPacket(*packet)     // resend
			packet.sentTime = now     // reset sent time
			packet.resendTimeout *= 2 // reset check timeout
			c.StatResend++
			c.Log("Resend %d at %d nextCheck %d", packet.serial, now, packet.resendTimeout)
		}
	}
}

func (c *Conn) Send(data []byte) error {
	// send
	packet := c.newPacket(data, ACK)
	err := c.sendPacket(packet)
	if err != nil {
		return err
	}
	// push to unackPackets
	packet.sentTime = c.ackCheckTimer.Now
	c.unackPackets.PushBack(&packet)
	// reset ackTimer
	c.ackTimer.Reset(ackTimerTimeout)
	// log
	c.Log("Send serial %d", packet.serial)
	return nil
}

func (c *Conn) sendAck() {
	packet := c.newPacket([]byte{}, ACK)
	c.sendPacket(packet)
	c.ackTimer.Reset(ackTimerTimeout)
	c.Log("Send Ack")
}
