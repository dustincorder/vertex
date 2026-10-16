package protocol

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"unicode/utf8"
)

func AppendInt32(dst []byte, v int32) []byte {
	var b [4]byte
	binary.BigEndian.PutUint32(b[:], uint32(v))
	return append(dst, b[:]...)
}

func AppendInt64(dst []byte, v int64) []byte {
	var b [8]byte
	binary.BigEndian.PutUint64(b[:], uint64(v))
	return append(dst, b[:]...)
}

func AppendFloat32(dst []byte, v float32) []byte {
	return AppendUint32(dst, math.Float32bits(v))
}

func AppendFloat64(dst []byte, v float64) []byte {
	return AppendInt64(dst, int64(math.Float64bits(v)))
}

func AppendUint32(dst []byte, v uint32) []byte {
	var b [4]byte
	binary.BigEndian.PutUint32(b[:], v)
	return append(dst, b[:]...)
}

func AppendPosition(dst []byte, x, y, z int32) []byte {
	packed := (uint64(x)&0x3ffffff)<<38 | (uint64(z)&0x3ffffff)<<12 | uint64(y)&0xfff
	return AppendInt64(dst, int64(packed))
}

var ErrMalformedPacket = errors.New("malformed Minecraft packet")

type Packet struct {
	ID   int32
	Data []byte
}

func Decode(frame []byte) (Packet, error) {
	r := bytes.NewReader(frame)
	id, err := ReadVarInt(r)
	if err != nil {
		return Packet{}, err
	}
	data := make([]byte, r.Len())
	_, _ = r.Read(data)
	return Packet{ID: id, Data: data}, nil
}

func Encode(id int32, data []byte) []byte {
	packet := AppendVarInt(make([]byte, 0, 5+len(data)), id)
	packet = append(packet, data...)
	return packet
}

func ReadString(r *bytes.Reader, maxBytes int) (string, error) {
	n, err := ReadVarInt(r)
	if err != nil || n < 0 || int(n) > maxBytes || int(n) > r.Len() {
		return "", ErrMalformedPacket
	}
	b := make([]byte, int(n))
	if _, err := r.Read(b); err != nil || !utf8.Valid(b) {
		return "", ErrMalformedPacket
	}
	return string(b), nil
}

func WriteString(dst []byte, value string) []byte {
	dst = AppendVarInt(dst, int32(len(value)))
	return append(dst, value...)
}

func DecodeHandshake(data []byte) (protocolVersion int32, host string, port uint16, nextState int32, err error) {
	r := bytes.NewReader(data)
	protocolVersion, err = ReadVarInt(r)
	if err != nil {
		err = ErrMalformedPacket
		return
	}
	if host, err = ReadString(r, 255); err != nil {
		return 0, "", 0, 0, err
	}
	if r.Len() < 3 {
		err = ErrMalformedPacket
		return
	}
	var raw [2]byte
	if _, err = r.Read(raw[:]); err != nil {
		err = ErrMalformedPacket
		return
	}
	port = binary.BigEndian.Uint16(raw[:])
	if nextState, err = ReadVarInt(r); err != nil || r.Len() != 0 || (nextState != 1 && nextState != 2 && nextState != 3) {
		err = fmt.Errorf("%w: invalid handshake state", ErrMalformedPacket)
	}
	return
}
