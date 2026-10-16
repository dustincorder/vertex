package protocol

import (
	"bytes"
	_ "embed"
	"encoding/binary"
	"testing"

	"github.com/ezchr/go-mcjava/chunk"
	"github.com/ezchr/go-mcjava/wire"
)

//go:embed data/v777/vanilla_flat_chunk_26.3.bin
var vanillaFlatChunk []byte

func TestFlatChunkBodyMatchesVanilla26_3Capture(t *testing.T) {
	capture := append([]byte(nil), vanillaFlatChunk...)
	body := FlatChunkBody(0, 0)
	// Captures may be at any coordinates; normalize the two leading ints and
	// compare the world data independently of the target light implementation.
	copy(capture[:8], body[:8])
	decoder := chunk.NewDecoder(chunk.VanillaBlockStates, chunk.VanillaBiomes)
	r := wire.NewReader(capture)
	var column chunk.Column
	if err := decoder.Decode(r, &column); err != nil {
		t.Fatal(err)
	}
	if r.Len() != 0 {
		t.Fatalf("decoder left %d bytes", r.Len())
	}
	if len(column.Sections) != 24 || column.Sections[0].BlockCount != 1024 {
		t.Fatalf("unexpected terrain shape")
	}
	if got := column.Sections[0].Blocks[chunk.BlockIndex(5, 3, 7)]; got != 9 {
		t.Fatalf("surface block state=%d, want grass_block=9", got)
	}
	// Compare all heightmaps, palettes, block states, and biomes. The outgoing
	// packet uses a simple full-bright sky mask instead of light propagation.
	var generated chunk.Column
	gr := wire.NewReader(body)
	if err := decoder.Decode(gr, &generated); err != nil {
		t.Fatal(err)
	}
	if len(generated.Sections) != len(column.Sections) {
		t.Fatalf("section count mismatch")
	}
	for i := range column.Sections {
		if generated.Sections[i].Blocks != column.Sections[i].Blocks || generated.Sections[i].Biomes != column.Sections[i].Biomes || generated.Sections[i].BlockCount != column.Sections[i].BlockCount {
			t.Fatalf("section %d differs from vanilla terrain", i)
		}
	}
	for i := range generated.Heightmaps {
		if generated.Heightmaps[i].Type != column.Heightmaps[i].Type || !bytes.Equal(uint64Bytes(generated.Heightmaps[i].Data), uint64Bytes(column.Heightmaps[i].Data)) {
			t.Fatalf("heightmap %d differs from vanilla", i)
		}
	}
}

func uint64Bytes(v []uint64) []byte {
	b := make([]byte, len(v)*8)
	for i, x := range v {
		binary.BigEndian.PutUint64(b[i*8:], x)
	}
	return b
}

func TestFlatChunkPacketBodyRewritesOnlyCoordinates(t *testing.T) {
	body := FlatChunkPacketBody(-2, 7)
	if got := int32(binary.BigEndian.Uint32(body[:4])); got != -2 {
		t.Fatalf("chunk x=%d", got)
	}
	if got := int32(binary.BigEndian.Uint32(body[4:8])); got != 7 {
		t.Fatalf("chunk z=%d", got)
	}
	if !bytes.Equal(body[8:], FlatChunkBody(0, 0)[8:]) {
		t.Fatal("chunk template body changed while rewriting coordinates")
	}
}
