package rand

import (
	"encoding/binary"
	"io"
)

type ReaderSource struct {
	Reader io.Reader
}

func (rs ReaderSource) Int63() int64 {
	return int64(rs.Uint64())
}

func (rs ReaderSource) Uint64() uint64 {
	var r uint64
	// TODO: this is slowish due to introspection
	if err := binary.Read(rs.Reader, binary.LittleEndian, &r); err != nil {
		panic(err)
	}
	return r
}

func (rs ReaderSource) Seed(int64) {
	panic("no seeding")
}
