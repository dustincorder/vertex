package protocol

import (
	"bytes"
	"crypto/md5"
	"crypto/rand"
	"errors"
	"regexp"
)

var validUsername = regexp.MustCompile(`^[A-Za-z0-9_]{1,16}$`)

type LoginHello struct {
	Name string
	UUID [16]byte
}

func DecodeLoginHello(data []byte) (LoginHello, error) {
	r := bytes.NewReader(data)
	name, err := ReadString(r, 64)
	if err != nil || !validUsername.MatchString(name) || r.Len() != 16 {
		return LoginHello{}, ErrMalformedPacket
	}
	var hello LoginHello
	if _, err := r.Read(hello.UUID[:]); err != nil {
		return LoginHello{}, ErrMalformedPacket
	}
	hello.Name = name
	return hello, nil
}

func OfflineUUID(name string) [16]byte {
	// Java's offline profile UUID is UUID.nameUUIDFromBytes(UTF8("OfflinePlayer:"+name)).
	id := md5.Sum([]byte("OfflinePlayer:" + name))
	id[6] = (id[6] & 0x0f) | 0x30
	id[8] = (id[8] & 0x3f) | 0x80
	return id
}

func LoginFinishedBody(name string, profileID, sessionID [16]byte) []byte {
	body := make([]byte, 0, 16+len(name)+24)
	body = append(body, profileID[:]...)
	body = WriteString(body, name)
	body = AppendVarInt(body, 0) // no signed texture/profile properties in offline mode
	body = append(body, sessionID[:]...)
	return body
}

func NewSessionID() [16]byte {
	var id [16]byte
	if _, err := rand.Read(id[:]); err != nil {
		id = md5.Sum([]byte("VertexSession:fallback"))
	}
	id[6] = (id[6] & 0x0f) | 0x40
	id[8] = (id[8] & 0x3f) | 0x80
	return id
}

var ErrUnexpectedState = errors.New("unexpected packet for protocol state")
