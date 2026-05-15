package main

import (
	"errors"
	"flag"
	"fmt"
	"image/color"
	"math"

	"github.com/tinyrange/gowin"
)

var errDone = errors.New("voxel demo complete")

const (
	chunkSize  = 12
	viewRadius = 2
	blockSize  = float32(1)
)

type chunkKey struct {
	X int
	Z int
}

type blockKey struct {
	X int
	Y int
	Z int
}

type chunk struct {
	key    chunkKey
	blocks map[blockKey]color.Color
	mesh   *gowin.Mesh
	draw   *gowin.DrawCommand
	node   *gowin.Node
	dirty  bool
}

type demo struct {
	maxFrames int
	scene     *gowin.Scene
	chunks    map[chunkKey]*chunk
	player    gowin.Vec3
	yaw       float32
	pitch     float32
	editSolid bool
}

func (d *demo) Init(ctx *gowin.Context) error {
	d.scene = gowin.NewScene()
	d.chunks = map[chunkKey]*chunk{}
	d.player = gowin.Vec3{X: 4, Y: 8, Z: 8}
	d.yaw = -0.8
	d.pitch = -0.18
	d.editSolid = true
	ctx.SetMouseCaptured(true)
	return d.streamChunks(ctx)
}

func (d *demo) Update(ctx *gowin.Context, dt float32) error {
	if d.maxFrames > 0 && int(ctx.Time().Frame) >= d.maxFrames {
		return errDone
	}
	if ctx.IsKeyPressed(gowin.KeyEscape) {
		ctx.SetMouseCaptured(false)
	}
	if ctx.IsButtonPressed(gowin.MouseLeft) {
		ctx.SetMouseCaptured(true)
	}

	if ctx.IsMouseCaptured() {
		mouse := ctx.MouseDelta()
		d.yaw += mouse.X * 0.0025
		d.pitch += mouse.Y * 0.0025
		d.pitch = clamp(d.pitch, -1.3, 1.3)
	}

	turn := float32(1.8) * dt
	if ctx.IsKeyDown(gowin.KeyQ) {
		d.yaw -= turn
	}
	if ctx.IsKeyDown(gowin.KeyE) {
		d.yaw += turn
	}

	forward := gowin.Vec3{X: float32(math.Sin(float64(d.yaw))), Z: float32(-math.Cos(float64(d.yaw)))}
	right := gowin.Vec3{X: -forward.Z, Z: forward.X}
	speed := float32(7) * dt
	if ctx.IsKeyDown(gowin.KeyW) {
		d.player = d.player.Add(forward.MulScalar(speed))
	}
	if ctx.IsKeyDown(gowin.KeyS) {
		d.player = d.player.Sub(forward.MulScalar(speed))
	}
	if ctx.IsKeyDown(gowin.KeyD) {
		d.player = d.player.Add(right.MulScalar(speed))
	}
	if ctx.IsKeyDown(gowin.KeyA) {
		d.player = d.player.Sub(right.MulScalar(speed))
	}
	if ctx.IsKeyDown(gowin.KeySpace) {
		d.player.Y += speed
	}
	if ctx.IsKeyDown(gowin.KeyF) {
		d.player.Y -= speed
	}
	if ctx.IsKeyPressed(gowin.KeyR) {
		if err := d.editBlock(ctx); err != nil {
			return err
		}
	}
	return d.streamChunks(ctx)
}

func (d *demo) Draw(ctx *gowin.Context) error {
	look := d.lookDirection()
	camera := gowin.Camera3D{
		Position: d.player,
		Target:   d.player.Add(look),
		Up:       gowin.Vec3{Y: 1},
		FOVY:     float32(math.Pi / 3),
		Near:     0.05,
		Far:      250,
	}
	ctx.Begin3D(camera)
	ctx.DrawScene(d.scene)
	ctx.End3D()

	ctx.DrawText("WASD move  mouse look  click capture  Esc release  Space/F up/down  R edit block", 16, 26, 16, color.White)
	ctx.DrawText(fmt.Sprintf("chunks: %d  pos: %.1f %.1f %.1f", len(d.chunks), d.player.X, d.player.Y, d.player.Z), 16, 50, 14, color.NRGBA{R: 212, G: 226, B: 235, A: 255})
	return nil
}

func (d *demo) lookDirection() gowin.Vec3 {
	cp := float32(math.Cos(float64(d.pitch)))
	return gowin.Vec3{
		X: float32(math.Sin(float64(d.yaw))) * cp,
		Y: float32(math.Sin(float64(d.pitch))),
		Z: float32(-math.Cos(float64(d.yaw))) * cp,
	}
}

func (d *demo) streamChunks(ctx *gowin.Context) error {
	center := worldChunk(d.player.X, d.player.Z)
	needed := map[chunkKey]bool{}
	for dz := -viewRadius; dz <= viewRadius; dz++ {
		for dx := -viewRadius; dx <= viewRadius; dx++ {
			key := chunkKey{X: center.X + dx, Z: center.Z + dz}
			needed[key] = true
			if _, ok := d.chunks[key]; ok {
				continue
			}
			c, err := d.newChunk(ctx, key)
			if err != nil {
				return err
			}
			d.chunks[key] = c
		}
	}
	for key, c := range d.chunks {
		if needed[key] {
			continue
		}
		if c.mesh != nil {
			c.mesh.Destroy()
		}
		if c.node != nil {
			c.node.SetDraw(nil)
		}
		delete(d.chunks, key)
	}
	return nil
}

func (d *demo) newChunk(ctx *gowin.Context, key chunkKey) (*chunk, error) {
	c := &chunk{key: key, blocks: map[blockKey]color.Color{}}
	for z := 0; z < chunkSize; z++ {
		for x := 0; x < chunkSize; x++ {
			wx := key.X*chunkSize + x
			wz := key.Z*chunkSize + z
			h := terrainHeight(wx, wz)
			for y := 0; y <= h; y++ {
				c.blocks[blockKey{X: x, Y: y, Z: z}] = blockColor(y, h)
			}
		}
	}
	if err := d.rebuildChunk(ctx, c); err != nil {
		return nil, err
	}
	c.node = d.scene.NewNode()
	c.node.SetDraw(c.draw)
	return c, nil
}

func (d *demo) editBlock(ctx *gowin.Context) error {
	key := worldChunk(d.player.X, d.player.Z)
	c := d.chunks[key]
	if c == nil {
		return nil
	}
	lx := positiveMod(int(math.Floor(float64(d.player.X))), chunkSize)
	lz := positiveMod(int(math.Floor(float64(d.player.Z))), chunkSize)
	y := terrainHeight(key.X*chunkSize+lx, key.Z*chunkSize+lz) + 1
	b := blockKey{X: lx, Y: y, Z: lz}
	if _, ok := c.blocks[b]; ok {
		delete(c.blocks, b)
	} else {
		c.blocks[b] = color.NRGBA{R: 220, G: 180, B: 86, A: 255}
	}
	return d.rebuildChunk(ctx, c)
}

func (d *demo) rebuildChunk(ctx *gowin.Context, c *chunk) error {
	data := buildChunkMesh(c)
	mesh := ctx.NewMesh(gowin.MeshOptions{Usage: gowin.DynamicMesh})
	if err := mesh.SetData(data); err != nil {
		return err
	}
	if c.mesh != nil {
		c.mesh.Destroy()
	}
	draw, err := ctx.PrepareDraw(mesh, gowin.DrawOptions{
		Transform: gowin.Translate3D(float32(c.key.X*chunkSize), 0, float32(c.key.Z*chunkSize)),
		Uniforms: gowin.Uniforms{
			"u_Ambient":        float32(0.34),
			"u_LightDirection": gowin.Vec3{X: -0.45, Y: 0.8, Z: 0.35},
		},
	})
	if err != nil {
		mesh.Destroy()
		return err
	}
	c.mesh = mesh
	c.draw = draw
	if c.node != nil {
		c.node.SetDraw(draw)
	}
	return nil
}

func buildChunkMesh(c *chunk) gowin.MeshData {
	var vertices []gowin.Vertex3D
	var indices []uint32
	for b, col := range c.blocks {
		for _, face := range cubeFaces {
			n := blockKey{X: b.X + int(face.normal.X), Y: b.Y + int(face.normal.Y), Z: b.Z + int(face.normal.Z)}
			if _, ok := c.blocks[n]; ok {
				continue
			}
			base := uint32(len(vertices))
			for _, p := range face.points {
				vertices = append(vertices, gowin.Vertex3D{
					Position: gowin.Vec3{
						X: float32(b.X) + p.X*blockSize,
						Y: float32(b.Y) + p.Y*blockSize,
						Z: float32(b.Z) + p.Z*blockSize,
					},
					Normal: face.normal,
					UV:     p.UV,
					Color:  col,
				})
			}
			indices = append(indices, base, base+1, base+2, base, base+2, base+3)
		}
	}
	return gowin.MeshData{Vertices: vertices, Indices: indices}
}

func terrainHeight(x, z int) int {
	wave := math.Sin(float64(x)*0.22) + math.Cos(float64(z)*0.18) + math.Sin(float64(x+z)*0.08)
	return 2 + int(math.Round(wave*1.5))
}

func blockColor(y, h int) color.Color {
	if y == h {
		return color.NRGBA{R: 74, G: 151, B: 84, A: 255}
	}
	if y > h-2 {
		return color.NRGBA{R: 116, G: 88, B: 60, A: 255}
	}
	return color.NRGBA{R: 92, G: 94, B: 105, A: 255}
}

func worldChunk(x, z float32) chunkKey {
	return chunkKey{
		X: floorDiv(int(math.Floor(float64(x))), chunkSize),
		Z: floorDiv(int(math.Floor(float64(z))), chunkSize),
	}
}

func floorDiv(v, d int) int {
	if v >= 0 {
		return v / d
	}
	return -((-v + d - 1) / d)
}

func positiveMod(v, d int) int {
	r := v % d
	if r < 0 {
		r += d
	}
	return r
}

func clamp(v, min, max float32) float32 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

type facePoint struct {
	X  float32
	Y  float32
	Z  float32
	UV gowin.Vec2
}

type cubeFace struct {
	normal gowin.Vec3
	points [4]facePoint
}

var cubeFaces = []cubeFace{
	{normal: gowin.Vec3{Z: 1}, points: [4]facePoint{{X: 0, Y: 0, Z: 1}, {X: 1, Y: 0, Z: 1, UV: gowin.Vec2{X: 1}}, {X: 1, Y: 1, Z: 1, UV: gowin.Vec2{X: 1, Y: 1}}, {X: 0, Y: 1, Z: 1, UV: gowin.Vec2{Y: 1}}}},
	{normal: gowin.Vec3{Z: -1}, points: [4]facePoint{{X: 1, Y: 0, Z: 0}, {X: 0, Y: 0, Z: 0, UV: gowin.Vec2{X: 1}}, {X: 0, Y: 1, Z: 0, UV: gowin.Vec2{X: 1, Y: 1}}, {X: 1, Y: 1, Z: 0, UV: gowin.Vec2{Y: 1}}}},
	{normal: gowin.Vec3{Y: 1}, points: [4]facePoint{{X: 0, Y: 1, Z: 1}, {X: 1, Y: 1, Z: 1, UV: gowin.Vec2{X: 1}}, {X: 1, Y: 1, Z: 0, UV: gowin.Vec2{X: 1, Y: 1}}, {X: 0, Y: 1, Z: 0, UV: gowin.Vec2{Y: 1}}}},
	{normal: gowin.Vec3{Y: -1}, points: [4]facePoint{{X: 0, Y: 0, Z: 0}, {X: 1, Y: 0, Z: 0, UV: gowin.Vec2{X: 1}}, {X: 1, Y: 0, Z: 1, UV: gowin.Vec2{X: 1, Y: 1}}, {X: 0, Y: 0, Z: 1, UV: gowin.Vec2{Y: 1}}}},
	{normal: gowin.Vec3{X: 1}, points: [4]facePoint{{X: 1, Y: 0, Z: 1}, {X: 1, Y: 0, Z: 0, UV: gowin.Vec2{X: 1}}, {X: 1, Y: 1, Z: 0, UV: gowin.Vec2{X: 1, Y: 1}}, {X: 1, Y: 1, Z: 1, UV: gowin.Vec2{Y: 1}}}},
	{normal: gowin.Vec3{X: -1}, points: [4]facePoint{{X: 0, Y: 0, Z: 0}, {X: 0, Y: 0, Z: 1, UV: gowin.Vec2{X: 1}}, {X: 0, Y: 1, Z: 1, UV: gowin.Vec2{X: 1, Y: 1}}, {X: 0, Y: 1, Z: 0, UV: gowin.Vec2{Y: 1}}}},
}

func main() {
	frames := flag.Int("frames", 0, "exit after this many frames; 0 runs interactively")
	flag.Parse()
	err := gowin.Run(&demo{maxFrames: *frames}, gowin.Config{
		Title:      "gowin voxel demo",
		Width:      1100,
		Height:     720,
		ClearColor: color.NRGBA{R: 92, G: 134, B: 172, A: 255},
	})
	if err != nil && !errors.Is(err, errDone) {
		panic(err)
	}
}
