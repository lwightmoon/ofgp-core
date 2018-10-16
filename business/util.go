package business

import (
	"encoding/binary"
	"io"
)

var bigEndian = binary.BigEndian

// Read 读取定长[]byte
func Read(r io.Reader, length int) []byte {
	res := make([]byte, length)
	io.ReadFull(r, res)
	return res
}

// ReadInt 读取int
func ReadInt(r io.Reader, element interface{}) error {
	var err error
	switch e := element.(type) {
	case *uint8:
		buf := make([]byte, 1)
		_, err = io.ReadFull(r, buf)
		*e = buf[0]
	case *uint32:
		buf := make([]byte, 4)
		_, err = io.ReadFull(r, buf)
		*e = bigEndian.Uint32(buf)
	case *uint64:
		buf := make([]byte, 8)
		_, err = io.ReadFull(r, buf)
		*e = bigEndian.Uint64(buf)
	}
	return err
}
