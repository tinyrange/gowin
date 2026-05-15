package main

import (
	"testing"

	"github.com/tinyrange/gowin"
)

func BenchmarkChunkGeneration16(b *testing.B) {
	spawn := findSpawnPoint()
	center := worldChunk(spawn.X, spawn.Y, spawn.Z)
	for i := 0; i < b.N; i++ {
		d := benchmarkDemo()
		c := d.newChunk(chunkKey{X: center.X + (i & 1), Y: center.Y, Z: center.Z + ((i >> 1) & 1)})
		if countChunkBlocks(c) == 0 {
			b.Fatal("generated empty chunk")
		}
	}
}

func countChunkBlocks(c *chunk) int {
	var n int
	for _, kind := range c.blocks {
		if kind != 0 {
			n++
		}
	}
	return n
}

func BenchmarkChunkMesh16WithNeighbors(b *testing.B) {
	spawn := findSpawnPoint()
	center := worldChunk(spawn.X, spawn.Y, spawn.Z)
	d := benchmarkWorld(center, 1, 1)
	key := center
	c := d.chunks[key]
	if c == nil {
		b.Fatal("missing center chunk")
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		data := d.buildChunkMesh(c)
		if len(data.Vertices) == 0 || len(data.Indices) == 0 {
			b.Fatal("generated empty mesh")
		}
	}
}

func BenchmarkFarViewCPUStream(b *testing.B) {
	const radius = 4
	spawn := findSpawnPoint()
	center := worldChunk(spawn.X, spawn.Y, spawn.Z)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		d := benchmarkWorld(center, radius, 3)
		var vertices int
		for _, c := range d.chunks {
			c.lod = lodForDistance(max(abs(c.key.X-center.X), max(abs(c.key.Y-center.Y), abs(c.key.Z-center.Z))))
			data := d.buildChunkMesh(c)
			vertices += len(data.Vertices)
		}
		if vertices == 0 {
			b.Fatal("generated empty view")
		}
	}
}

func benchmarkDemo() *demo {
	return &demo{
		scene:  gowin.NewScene(),
		chunks: map[chunkKey]*chunk{},
	}
}

func benchmarkWorld(center chunkKey, radius, vertical int) *demo {
	d := benchmarkDemo()
	for y := -vertical; y <= vertical; y++ {
		for z := -radius; z <= radius; z++ {
			for x := -radius; x <= radius; x++ {
				key := chunkKey{X: center.X + x, Y: center.Y + y, Z: center.Z + z}
				d.chunks[key] = d.newChunk(key)
			}
		}
	}
	return d
}
