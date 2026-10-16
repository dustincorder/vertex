package world

// Chunk is a compact generated terrain column. Block ids are Vertex-internal
// palette values; protocol registry mapping belongs to the network serializer.
type Chunk struct {
	X, Z int32
}

const (
	MinY       = -64
	MaxY       = 320
	SurfaceY   = 63
	Air        = uint16(0)
	Stone      = uint16(1)
	Dirt       = uint16(2)
	GrassBlock = uint16(3)
	Bedrock    = uint16(4)
)

func GenerateChunk(x, z int32) *Chunk {
	return &Chunk{X: x, Z: z}
}

// BlockAt computes the deterministic flat terrain without allocating a block
// array per chunk. The packet encoder can build the required palettes directly.
func (c *Chunk) BlockAt(x, y, z int) uint16 {
	if x < 0 || x >= 16 || z < 0 || z >= 16 || y < MinY || y >= MaxY || y > SurfaceY {
		return Air
	}
	switch {
	case y == MinY:
		return Bedrock
	case y == SurfaceY:
		return GrassBlock
	case y >= SurfaceY-3:
		return Dirt
	default:
		return Stone
	}
}
