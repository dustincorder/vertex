package server

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"encoding/json"
	"math"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"vertex/internal/protocol"
)

const ProtocolVersion = 777
const MinecraftVersion = "26.3"

type Server struct {
	addr    string
	players atomic.Int32
	clients chan struct{}
}

func New(addr string) *Server {
	return &Server{addr: addr, clients: make(chan struct{}, 16)}
}

func (s *Server) ListenAndServe() error {
	ln, err := net.Listen("tcp", s.addr)
	if err != nil {
		return err
	}
	defer ln.Close()
	for {
		conn, err := ln.Accept()
		if err != nil {
			return err
		}
		select {
		case s.clients <- struct{}{}:
			go func() {
				defer func() { <-s.clients }()
				s.serveConn(conn)
			}()
		default:
			_ = conn.Close()
		}
	}
}

func (s *Server) serveConn(conn net.Conn) {
	defer conn.Close()
	if tcp, ok := conn.(*net.TCPConn); ok {
		_ = tcp.SetNoDelay(true)
	}
	_ = conn.SetDeadline(time.Now().Add(15 * time.Second))
	r := bufio.NewReaderSize(conn, 4096)
	first, err := protocol.ReadFrame(r)
	if err != nil {
		return
	}
	p, err := protocol.Decode(first)
	if err != nil || p.ID != 0 {
		return
	}
	_, _, _, state, err := protocol.DecodeHandshake(p.Data)
	if err != nil {
		return
	}

	switch state {
	case 1:
		s.serveStatus(conn, r)
	case 2:
		if err := s.serveLogin(conn, r); err != nil {
			return
		}
	case 3:
		return
	}
}

func (s *Server) serveLogin(conn net.Conn, r *bufio.Reader) error {
	_ = conn.SetDeadline(time.Now().Add(45 * time.Second))
	frame, err := protocol.ReadFrame(r)
	if err != nil {
		return err
	}
	p, err := protocol.Decode(frame)
	if err != nil || p.ID != 0 {
		return protocol.ErrUnexpectedState
	}
	hello, err := protocol.DecodeLoginHello(p.Data)
	if err != nil {
		return err
	}

	profileID := protocol.OfflineUUID(hello.Name)
	sessionID := protocol.NewSessionID()
	if err := protocol.WriteFrame(conn, protocol.Encode(0x02,
		protocol.LoginFinishedBody(hello.Name, profileID, sessionID))); err != nil {
		return err
	}

	frame, err = protocol.ReadFrame(r)
	if err != nil {
		return err
	}
	p, err = protocol.Decode(frame)
	if err != nil || p.ID != 0x03 || len(p.Data) != 0 {
		return protocol.ErrUnexpectedState
	}

	packets, err := protocol.ConfigurationPackets()
	if err != nil {
		return err
	}
	for _, packet := range packets {
		if err := protocol.WriteFrame(conn, protocol.Encode(packet.ID, packet.Body)); err != nil {
			return err
		}
	}
	if err := protocol.WriteFrame(conn, protocol.Encode(0x03, nil)); err != nil {
		return err
	}

	for {
		frame, err = protocol.ReadFrame(r)
		if err != nil {
			return err
		}
		p, err = protocol.Decode(frame)
		if err != nil {
			return err
		}
		switch p.ID {
		case 0x03: // client finished configuration; next protocol is Play
			if len(p.Data) != 0 {
				return protocol.ErrMalformedPacket
			}
			s.players.Add(1)
			defer s.players.Add(-1)
			return s.servePlay(conn, r, hello.Name, profileID)
		case 0x04: // keep-alive: echo unchanged while synchronizing registries
			if err := protocol.WriteFrame(conn, protocol.Encode(0x04, p.Data)); err != nil {
				return err
			}
		case 0x00: // client information is one-shot; retain future renegotiation support
		case 0x01, 0x02, 0x06, 0x07, 0x08, 0x09:
			// cookies, extension payloads, resource packs, known packs and conduct
			// acceptance do not change world state for this minimal core.
		case 0x05: // configuration ping
			if len(p.Data) != 4 {
				return protocol.ErrMalformedPacket
			}
			if err := protocol.WriteFrame(conn, protocol.Encode(0x05, p.Data)); err != nil {
				return err
			}
		default:
			return protocol.ErrUnexpectedState
		}
	}
}

func (s *Server) servePlay(conn net.Conn, r *bufio.Reader, name string, profileID [16]byte) error {
	_ = conn.SetDeadline(time.Time{})
	const radius int32 = 2
	var writeMu sync.Mutex
	write := func(id int32, body []byte) error {
		writeMu.Lock()
		defer writeMu.Unlock()
		return protocol.WriteFrame(conn, protocol.Encode(id, body))
	}
	if err := write(protocol.PlayLogin, protocol.PlayLoginBody(1, radius)); err != nil {
		return err
	}
	if err := write(protocol.PlaySpawnPosition, protocol.SpawnPositionBody(0, -60, 0)); err != nil {
		return err
	}
	if err := write(protocol.PlayPosition, protocol.PlayerPositionBody(1, .5, -60, .5)); err != nil {
		return err
	}
	if err := write(protocol.PlayGameEvent, []byte{13, 0, 0, 0, 0}); err != nil {
		return err
	}
	center := protocol.AppendVarInt(nil, 0)
	center = protocol.AppendVarInt(center, 0)
	if err := write(protocol.PlayChunkCacheCenter, center); err != nil {
		return err
	}
	if err := sendChunkArea(write, 0, 0, radius, nil); err != nil {
		return err
	}
	_, _ = name, profileID
	chunkX, chunkZ := int32(0), int32(0)

	closed := make(chan struct{})
	defer close(closed)
	go func() {
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-closed:
				return
			case now := <-ticker.C:
				if err := write(protocol.PlayKeepAlive, protocol.AppendInt64(nil, now.UnixMilli())); err != nil {
					return
				}
			}
		}
	}()
	for {
		_ = conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		frame, err := protocol.ReadFrame(r)
		if err != nil {
			return err
		}
		p, err := protocol.Decode(frame)
		if err != nil {
			return err
		}
		switch p.ID {
		case 0x00: // accept teleport
		case 0x0b: // chunk batch received
		case 0x0d: // client tick end
		case 0x1e, 0x1f: // position and position+rotation
			x, _, z, err := decodeMovePosition(p)
			if err != nil {
				return err
			}
			nx, nz := int32(math.Floor(x/16)), int32(math.Floor(z/16))
			if nx != chunkX || nz != chunkZ {
				oldX, oldZ := chunkX, chunkZ
				chunkX, chunkZ = nx, nz
				if err := write(protocol.PlayChunkCacheCenter, append(protocol.AppendVarInt(nil, nx), protocol.AppendVarInt(nil, nz)...)); err != nil {
					return err
				}
				if err := forgetChunkArea(write, oldX, oldZ, radius, nx, nz); err != nil {
					return err
				}
				if err := sendChunkArea(write, nx, nz, radius, func(x, z int32) bool {
					return x < oldX-radius || x > oldX+radius || z < oldZ-radius || z > oldZ+radius
				}); err != nil {
					return err
				}
			}
		case 0x20, 0x21: // rotation-only, on-ground-only
		case 0x2d: // keep-alive response
		case 0x09: // chat (not yet broadcast)
		case 0x0e: // client information
		case 0x10: // configuration acknowledged
		default:
			if p.ID < 0 || p.ID > 0x8f {
				return protocol.ErrUnexpectedState
			}
		}
	}
}

type playWriter func(int32, []byte) error

func sendChunkArea(write playWriter, centerX, centerZ, radius int32, include func(int32, int32) bool) error {
	chunks := make([][2]int32, 0, int((radius*2+1)*(radius*2+1)))
	for x := centerX - radius; x <= centerX+radius; x++ {
		for z := centerZ - radius; z <= centerZ+radius; z++ {
			if include == nil || include(x, z) {
				chunks = append(chunks, [2]int32{x, z})
			}
		}
	}
	if len(chunks) == 0 {
		return nil
	}
	if err := write(protocol.PlayChunkBatchStart, nil); err != nil {
		return err
	}
	for _, pos := range chunks {
		if err := write(protocol.PlayChunkWithLight, protocol.FlatChunkPacketBody(pos[0], pos[1])); err != nil {
			return err
		}
	}
	return write(protocol.PlayChunkBatchFinished, protocol.AppendVarInt(nil, int32(len(chunks))))
}

func decodeMovePosition(p protocol.Packet) (x, y, z float64, err error) {
	if len(p.Data) < 25 {
		return 0, 0, 0, protocol.ErrMalformedPacket
	}
	r := bytes.NewReader(p.Data)
	var raw uint64
	if err = binary.Read(r, binary.BigEndian, &raw); err != nil {
		return
	}
	x = math.Float64frombits(raw)
	if err = binary.Read(r, binary.BigEndian, &raw); err != nil {
		return
	}
	y = math.Float64frombits(raw)
	if err = binary.Read(r, binary.BigEndian, &raw); err != nil {
		return
	}
	z = math.Float64frombits(raw)
	if math.IsNaN(x) || math.IsNaN(y) || math.IsNaN(z) || math.Abs(x) > 30_000_000 || math.Abs(z) > 30_000_000 || y < -1000 || y > 10000 {
		err = protocol.ErrMalformedPacket
	}
	return
}

func forgetChunkArea(write playWriter, oldX, oldZ, radius, newX, newZ int32) error {
	for x := oldX - radius; x <= oldX+radius; x++ {
		for z := oldZ - radius; z <= oldZ+radius; z++ {
			if x >= newX-radius && x <= newX+radius && z >= newZ-radius && z <= newZ+radius {
				continue
			}
			if err := write(protocol.PlayForgetChunk, protocol.ForgetChunkBody(x, z)); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *Server) serveStatus(conn net.Conn, r *bufio.Reader) {
	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
	frame, err := protocol.ReadFrame(r)
	if err != nil {
		return
	}
	p, err := protocol.Decode(frame)
	if err != nil {
		return
	}
	switch p.ID {
	case 0:
		response := map[string]any{
			"version":     map[string]any{"name": MinecraftVersion, "protocol": ProtocolVersion},
			"players":     map[string]any{"max": 4, "online": s.players.Load(), "sample": []any{}},
			"description": map[string]string{"text": "Vertex experimental core"},
		}
		body, _ := json.Marshal(response)
		payload := protocol.WriteString(nil, string(body))
		_ = protocol.WriteFrame(conn, protocol.Encode(0, payload))
	case 1:
		if len(p.Data) != 8 {
			return
		}
		pong := protocol.Encode(1, p.Data)
		_ = protocol.WriteFrame(conn, pong)
	}
}
