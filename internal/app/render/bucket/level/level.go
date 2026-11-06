package level

import (
	"sort"

	"sdmm/internal/app/render/bucket/level/chunk"
	"sdmm/internal/dmapi/dmmap"
	"sdmm/internal/util"
)

// Level stores Chunk's data for a specific Z-level of the map.
type Level struct {
	value int

	// Chunks is a slice of all chunks on the level.
	Chunks map[util.Point]*chunk.Chunk
	// Layers stores all available layers for the level.
	Layers []float32
	// ChunksByLayers is the map which helps to find chunks with units on the specific layer.
	ChunksByLayers map[float32][]*chunk.Chunk
}

func New(dmm *dmmap.Dmm, level int) *Level {
	return &Level{
		value:  level,
		Chunks: generateChunks(dmm.MaxX, dmm.MaxY, dmmap.WorldIconSize),
	}
}

// Update updates current level chunks data.
// If tilesToUpdate is not nil, then only chunks with provided tiles will be updated.
func (l *Level) Update(dmm *dmmap.Dmm, tilesToUpdate []util.Point) {
	if tilesToUpdate != nil {
		// Store a set of updated chunks to avoid multiple updates for the same chunk area.
		updatedChunks := make(map[*chunk.Chunk]bool)

		for _, tile := range tilesToUpdate {
			// Only update chunk with an updated tile.
			c, ok := l.Chunks[findChunkBounds(tile.X, tile.Y)]
			// Tiles out of the map (e.g. after the map resize) have no chunk.
			if !ok || updatedChunks[c] {
				continue
			}

			c.Update(dmm, l.value)
			updatedChunks[c] = true
		}
	} else {
		// Update all available chunks.
		for _, c := range l.Chunks {
			c.Update(dmm, l.value)
		}
	}

	l.createChunksLayers()
}

func findChunkBounds(x, y int) util.Point {
	return util.Point{X: findChunkBound(x), Y: findChunkBound(y)}
}

func findChunkBound(value int) int {
	if value < 1 {
		return 1
	}
	// Chunks start at 1 and have (chunk.Size + 1) tiles per axis.
	return ((value-1)/(chunk.Size+1))*(chunk.Size+1) + 1
}

// Method collects layers for every unit in every chunk.
func (l *Level) createChunksLayers() {
	chunksByLayers := make(map[float32][]*chunk.Chunk, len(l.ChunksByLayers))
	for _, c := range l.Chunks {
		for chunkLayer := range c.UnitsByLayers {
			chunksByLayers[chunkLayer] = append(chunksByLayers[chunkLayer], c)
		}
	}

	// Sort layers to do a proper rendering later.
	layers := make([]float32, 0, len(chunksByLayers))
	for layer := range chunksByLayers {
		if len(chunksByLayers[layer]) > 0 {
			layers = append(layers, layer)
		}
	}
	sort.Slice(layers, func(i, j int) bool { return layers[i] < layers[j] })

	l.Layers = layers
	l.ChunksByLayers = chunksByLayers
}
