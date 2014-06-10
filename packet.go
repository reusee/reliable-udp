package udp

import (
	"bytes"
	"encoding/binary"
	"io/ioutil"
)

const (
	SYNC   = byte(1)
	FINISH = byte(2)
	ACK    = byte(4)
)

type Packet struct {
	serial uint32
	flag   byte
	data   []byte
}

func (c *Conn) newPacket(data []byte, flags ...byte) Packet {
	var flag byte
	for _, f := range flags {
		flag |= f
	}
	packet := Packet{
		serial: c.serial,
		flag:   flag,
		data:   data,
	}
	c.serial++
	c.Log("newPacket serial %d flag %x length %d", packet.serial, packet.flag, len(packet.data))
	return packet
}

func readPacket(packetData []byte) (uint32, uint32, byte, uint16, []byte) {
	reader := bytes.NewReader(packetData)
	var serial, ackSerial uint32
	binary.Read(reader, binary.LittleEndian, &serial)
	binary.Read(reader, binary.LittleEndian, &ackSerial)
	flag, _ := reader.ReadByte()
	var windowSize uint16
	binary.Read(reader, binary.LittleEndian, &windowSize)
	data, _ := ioutil.ReadAll(reader)
	return serial, ackSerial, flag, windowSize, data
}
