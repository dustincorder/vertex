package protocol

import "encoding/binary"

// Play packet ids are positional in the 26.3 (protocol 777) packet registry.
const (
	PlayChunkBatchFinished = 0x0b
	PlayChunkBatchStart    = 0x0c
	PlayKeepAlive          = 0x2d
	PlayChunkWithLight     = 0x2e
	PlayForgetChunk        = 0x26
	PlayLogin              = 0x32
	PlayPosition           = 0x49
	PlayChunkCacheCenter   = 0x60
	PlaySpawnPosition      = 0x63
	PlayGameEvent          = 0x27
)

// PlayLoginBody encodes the 26.3 CommonPlayerSpawnInfo and login fields.
func PlayLoginBody(entityID int32, radius int32) []byte {
	b := AppendInt32(nil, entityID)
	b = append(b, 0) // hardcore
	b = AppendVarInt(b, 1)
	b = WriteString(b, "minecraft:overworld")
	b = AppendVarInt(b, 4)
	b = AppendVarInt(b, radius)
	b = AppendVarInt(b, radius)
	b = append(b, 0, 1, 0) // reduced debug, death screen, limited crafting
	b = AppendVarInt(b, 0) // dimension type: overworld
	b = WriteString(b, "minecraft:overworld")
	b = AppendInt64(b, 0)    // hashed seed
	b = AppendVarInt(b, 1)   // creative
	b = AppendVarInt(b, 0)   // no previous game mode
	b = append(b, 0, 1, 0)   // debug, flat, no last death location
	b = AppendVarInt(b, 0)   // portal cooldown
	b = AppendVarInt(b, -63) // sea level
	b = append(b, 0, 0)      // offline, secure chat not enforced
	return b
}

func SpawnPositionBody(x, y, z int32) []byte {
	b := WriteString(nil, "minecraft:overworld")
	b = AppendPosition(b, x, y, z)
	return AppendFloat32(b, 0)
}

func PlayerPositionBody(teleportID int32, x, y, z float64) []byte {
	b := AppendVarInt(nil, teleportID)
	b = AppendFloat64(b, x)
	b = AppendFloat64(b, y)
	b = AppendFloat64(b, z)
	for range 3 {
		b = AppendFloat64(b, 0)
	}
	b = AppendFloat32(b, 0)
	b = AppendFloat32(b, 0)
	b = AppendInt32(b, 0) // absolute flags
	return b
}

// FlatChunkBody serializes a 26.3 flat overworld chunk. State ids are from the
// 26.3 vanilla registry: stone=1, grass_block=9, dirt=10, bedrock=88, plains=41.
func FlatChunkBody(chunkX, chunkZ int32) []byte {
	b := AppendInt32(nil, chunkX)
	b = AppendInt32(b, chunkZ)
	b = AppendVarInt(b, 3)
	for _, typ := range []int32{1, 4, 5} {
		b = AppendVarInt(b, typ)
		b = AppendVarInt(b, 37)
		var heights [256]uint16
		for i := range heights {
			heights[i] = 4 // first free block y=-60, relative to min y=-64
		}
		longs := packHeightmap(heights[:], 9)
		for _, v := range longs {
			b = AppendInt64(b, int64(v))
		}
	}

	sections := make([]byte, 0, 24*32)
	for sectionY := 0; sectionY < 24; sectionY++ {
		nonAir := 0
		palette := []int32{0}
		switch sectionY {
		case 0:
			nonAir, palette = 1024, []int32{88, 10, 9, 0}
		}
		sections = append(sections, byte(nonAir>>8), byte(nonAir), 0, 0) // block and fluid counts
		sections = append(sections, 0)                                   // single-value / indirect palette bit width
		switch sectionY {
		case 0:
			sections[len(sections)-1] = 4
			sections = AppendVarInt(sections, 4)
			for _, state := range palette {
				sections = AppendVarInt(sections, state)
			}
			data := make([]uint64, 256)
			for z := range 16 {
				for x := range 16 {
					for y := range 16 {
						idx := y<<8 | z<<4 | x
						paletteIndex := uint64(3)
						switch y {
						case 0:
							paletteIndex = 0
						case 1, 2:
							paletteIndex = 1
						case 3:
							paletteIndex = 2
						}
						data[idx/16] |= paletteIndex << (uint(idx%16) * 4)
					}
				}
			}
			for _, word := range data {
				sections = AppendInt64(sections, int64(word))
			}
		default:
			sections = AppendVarInt(sections, 0)
		}
		sections = append(sections, 0) // single-value plains biome
		sections = AppendVarInt(sections, 41)
	}
	b = AppendVarInt(b, int32(len(sections)))
	b = append(b, sections...)
	b = AppendVarInt(b, 0) // block entities

	// Flat terrain has clear daylight. Full-bright sky layers avoid requiring a
	// light propagation engine while keeping the surface visible at noon.
	b = appendBitSet(b, 0x03ffffff)
	b = appendBitSet(b, 0)
	b = appendBitSet(b, 0)
	b = appendBitSet(b, 0x03ffffff)
	b = AppendVarInt(b, 26)
	light := make([]byte, 2048)
	for i := range light {
		light[i] = 0xff
	}
	for range 26 {
		b = AppendVarInt(b, 2048)
		b = append(b, light...)
	}
	b = AppendVarInt(b, 0)
	b = AppendVarInt(b, 0) // block light arrays
	return b
}

var flatChunkTemplate = FlatChunkBody(0, 0)

func FlatChunkPacketBody(chunkX, chunkZ int32) []byte {
	b := append([]byte(nil), flatChunkTemplate...)
	binary.BigEndian.PutUint32(b[0:4], uint32(chunkX))
	binary.BigEndian.PutUint32(b[4:8], uint32(chunkZ))
	return b
}

func ForgetChunkBody(chunkX, chunkZ int32) []byte {
	b := AppendInt32(nil, chunkX)
	return AppendInt32(b, chunkZ)
}

func packHeightmap(v []uint16, bitsPerValue uint) []uint64 {
	per := int(64 / bitsPerValue)
	out := make([]uint64, (len(v)+per-1)/per)
	mask := uint64(1<<bitsPerValue) - 1
	for i, value := range v {
		out[i/per] |= (uint64(value) & mask) << (uint(i%per) * bitsPerValue)
	}
	return out
}

func appendBitSet(dst []byte, bits uint32) []byte {
	n := 0
	for v := bits; v != 0; v >>= 8 {
		n++
	}
	dst = AppendVarInt(dst, int32(n))
	for i := range n {
		dst = append(dst, byte(bits>>(8*i)))
	}
	return dst
}
