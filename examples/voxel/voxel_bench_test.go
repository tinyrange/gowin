package main

import (
	"testing"

	"github.com/tinyrange/gowin"
)

func BenchmarkChunkGeneration16(b *testing.B) {
	for i := 0; i < b.N; i++ {
		d := benchmarkDemo()
		c := d.newChunk(chunkKey{X: i & 15, Y: 0, Z: (i >> 4) & 15})
		if len(c.blocks) == 0 {
			b.Fatal("generated empty chunk")
		}
	}
}

func BenchmarkChunkMesh16WithNeighbors(b *testing.B) {
	d := benchmarkWorld(1, 1)
	key := chunkKey{}
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
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		d := benchmarkWorld(radius, verticalRadius)
		var vertices int
		for _, c := range d.chunks {
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

func benchmarkWorld(radius, vertical int) *demo {
	d := benchmarkDemo()
	for y := -vertical; y <= vertical; y++ {
		for z := -radius; z <= radius; z++ {
			for x := -radius; x <= radius; x++ {
				key := chunkKey{X: x, Y: y, Z: z}
				d.chunks[key] = d.newChunk(key)
			}
		}
	}
	return d
}
