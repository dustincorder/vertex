package protocol

import (
	"bufio"
	"errors"
	"io"
)

const MaxFrameSize = 2 << 20

var ErrInvalidFrame = errors.New("invalid Minecraft frame size")

func ReadFrame(r *bufio.Reader) ([]byte, error) {
	n, err := ReadVarInt(r)
	if err != nil {
		return nil, err
	}
	if n < 1 || n > MaxFrameSize {
		return nil, ErrInvalidFrame
	}
	frame := make([]byte, int(n))
	if _, err := io.ReadFull(r, frame); err != nil {
		return nil, err
	}
	return frame, nil
}

func WriteFrame(w io.Writer, payload []byte) error {
	if len(payload) < 1 || len(payload) > MaxFrameSize {
		return ErrInvalidFrame
	}
	out := AppendVarInt(make([]byte, 0, 5+len(payload)), int32(len(payload)))
	out = append(out, payload...)
	for len(out) > 0 {
		n, err := w.Write(out)
		if err != nil {
			return err
		}
		if n == 0 {
			return io.ErrShortWrite
		}
		out = out[n:]
	}
	return nil
}
