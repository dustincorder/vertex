package protocol

import (
	"errors"
	"io"
)

var ErrVarIntTooLong = errors.New("minecraft VarInt exceeds 5 bytes")

func ReadVarInt(r io.ByteReader) (int32, error) {
	var value uint32
	for i := 0; i < 5; i++ {
		b, err := r.ReadByte()
		if err != nil {
			return 0, err
		}
		value |= uint32(b&0x7f) << (7 * i)
		if b&0x80 == 0 {
			return int32(value), nil
		}
	}
	return 0, ErrVarIntTooLong
}

func AppendVarInt(dst []byte, value int32) []byte {
	u := uint32(value)
	for u >= 0x80 {
		dst = append(dst, byte(u)|0x80)
		u >>= 7
	}
	return append(dst, byte(u))
}
