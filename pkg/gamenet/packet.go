package gamenet

import (
	"bytes"
	"encoding/binary"
	"errors"

	"github.com/karashiiro/kartlobby/pkg/network"
)

func ReadPacket(data []byte, s interface{}) error {
	buf := bytes.NewReader(data)
	binary.Read(buf, binary.LittleEndian, s)

	if header, ok := s.(*PacketHeader); ok && header.Checksum != netBufferChecksum(data) {
		return errors.New("checksum mismatch")
	}

	return nil
}

func SendPacket(conn network.Connection, data interface{}) error {
	buf := new(bytes.Buffer)
	err := binary.Write(buf, binary.LittleEndian, data)
	if err != nil {
		return err
	}

	sendBuf := buf.Bytes()
	checksum := netBufferChecksum(sendBuf[4:])
	binary.LittleEndian.PutUint32(sendBuf[0:4], checksum)

	err = conn.Send(sendBuf)
	if err != nil {
		return err
	}

	return nil
}

func netBufferChecksum(buf []byte) uint32 {
	checksum := uint32(0x1234567)
	length := len(buf) - 4
	for i := 0; i < length; i++ {
		checksum += uint32(buf[i]) * uint32(i+1)
	}
	return checksum
}
