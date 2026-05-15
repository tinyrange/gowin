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
	chunkSize    = 12
	viewRadius   = 2
	playerRadius = float32(0.32)
	playerHeight = float32(1.78)
	eyeHeight    = float32(1.58)
	gravity      = float32(24)
	jumpSpeed    = float32(8.2)
	walkSpeed    = float32(5.2)
	sprintSpeed  = float32(8.4)
	reach        = float32(7)
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

type blockKind uint8

const (
	blockGrass blockKind = iota + 1
	blockDirt
	blockStone
	blockWood
	blockLeaves
	blockSand
)

type chunk struct {
	key    chunkKey
	blocks map[blockKey]blockKind
	mesh   *gowin.Mesh
	draw   *gowin.DrawCommand
	node   *gowin.Node
}

type rayHit struct {
	block  worldBlock
	place  worldBlock
	normal gowin.Vec3
}

type worldBlock struct {
	X int
	Y int
	Z int
}

type demo struct {
	maxFrames int
	scene     *gowin.Scene
	chunks    map[chunkKey]*chunk

	player   gowin.Vec3 // feet position
	vel      gowin.Vec3
	yaw      float32
	pitch    float32
	onGround bool

	selection     *rayHit
	selectionMesh *gowin.Mesh
	selectionDraw *gowin.DrawCommand
}

func (d *demo) Init(ctx *gowin.Context) error {
	d.scene = gowin.NewScene()
	d.chunks = map[chunkKey]*chunk{}
	d.player = gowin.Vec3{X: 4.5, Y: float32(terrainHeight(4, 8) + 3), Z: 8.5}
	d.yaw = -0.8
	d.pitch = -0.15
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
	if !ctx.IsMouseCaptured() {
		if ctx.IsButtonPressed(gowin.MouseLeft) || ctx.IsButtonPressed(gowin.MouseRight) {
			ctx.SetMouseCaptured(true)
		}
		return d.streamChunks(ctx)
	}

	mouse := ctx.MouseDelta()
	d.yaw += mouse.X * 0.0025
	d.pitch -= mouse.Y * 0.0025
	d.pitch = clamp(d.pitch, -1.35, 1.35)

	if ctx.IsKeyDown(gowin.KeyQ) {
		d.yaw -= 1.8 * dt
	}
	if ctx.IsKeyDown(gowin.KeyE) {
		d.yaw += 1.8 * dt
	}

	if err := d.updatePhysics(ctx, dt); err != nil {
		return err
	}
	if err := d.streamChunks(ctx); err != nil {
		return err
	}
	if err := d.updateSelection(ctx); err != nil {
		return err
	}
	if ctx.IsButtonPressed(gowin.MouseLeft) {
		return d.deleteSelection(ctx)
	}
	if ctx.IsButtonPressed(gowin.MouseRight) {
		return d.placeSelection(ctx)
	}
	return nil
}

func (d *demo) updatePhysics(ctx *gowin.Context, dt float32) error {
	forward := gowin.Vec3{X: float32(math.Sin(float64(d.yaw))), Z: float32(-math.Cos(float64(d.yaw)))}
	right := gowin.Vec3{X: -forward.Z, Z: forward.X}
	wish := gowin.Vec3{}
	if ctx.IsKeyDown(gowin.KeyW) {
		wish = wish.Add(forward)
	}
	if ctx.IsKeyDown(gowin.KeyS) {
		wish = wish.Sub(forward)
	}
	if ctx.IsKeyDown(gowin.KeyD) {
		wish = wish.Add(right)
	}
	if ctx.IsKeyDown(gowin.KeyA) {
		wish = wish.Sub(right)
	}
	speed := walkSpeed
	if ctx.IsKeyDown(gowin.KeyShift) || ctx.IsKeyDown(gowin.KeyRShift) {
		speed = sprintSpeed
	}
	wish = wish.Normalize().MulScalar(speed)
	d.vel.X = wish.X
	d.vel.Z = wish.Z

	if ctx.IsKeyPressed(gowin.KeySpace) && d.onGround {
		d.vel.Y = jumpSpeed
		d.onGround = false
	}
	d.vel.Y -= gravity * dt

	d.moveAxis('x', d.vel.X*dt)
	d.moveAxis('z', d.vel.Z*dt)
	d.onGround = false
	d.moveAxis('y', d.vel.Y*dt)
	if d.player.Y < -16 {
		d.player = gowin.Vec3{X: 4.5, Y: float32(terrainHeight(4, 8) + 4), Z: 8.5}
		d.vel = gowin.Vec3{}
	}
	return nil
}

func (d *demo) moveAxis(axis byte, amount float32) {
	if amount == 0 {
		return
	}
	next := d.player
	switch axis {
	case 'x':
		next.X += amount
	case 'y':
		next.Y += amount
	case 'z':
		next.Z += amount
	}
	if !d.collides(next) {
		d.player = next
		return
	}
	switch axis {
	case 'x':
		d.vel.X = 0
	case 'z':
		d.vel.Z = 0
	case 'y':
		if amount < 0 {
			d.onGround = true
		}
		d.vel.Y = 0
	}
}

func (d *demo) collides(pos gowin.Vec3) bool {
	minX := int(math.Floor(float64(pos.X - playerRadius)))
	maxX := int(math.Floor(float64(pos.X + playerRadius)))
	minY := int(math.Floor(float64(pos.Y)))
	maxY := int(math.Floor(float64(pos.Y + playerHeight)))
	minZ := int(math.Floor(float64(pos.Z - playerRadius)))
	maxZ := int(math.Floor(float64(pos.Z + playerRadius)))
	for y := minY; y <= maxY; y++ {
		for z := minZ; z <= maxZ; z++ {
			for x := minX; x <= maxX; x++ {
				if d.blockAt(worldBlock{X: x, Y: y, Z: z}) != 0 {
					return true
				}
			}
		}
	}
	return false
}

func (d *demo) Draw(ctx *gowin.Context) error {
	eye := d.eyePosition()
	camera := gowin.Camera3D{
		Position: eye,
		Target:   eye.Add(d.lookDirection()),
		Up:       gowin.Vec3{Y: 1},
		FOVY:     float32(math.Pi / 3),
		Near:     0.05,
		Far:      250,
	}
	ctx.Begin3D(camera)
	ctx.DrawScene(d.scene)
	if d.selectionDraw != nil {
		ctx.Draw(d.selectionDraw)
	}
	ctx.End3D()

	ctx.DrawText("WASD move  Shift sprint  Space jump  mouse look  LMB delete  RMB place  Esc release", 16, 26, 16, color.White)
	ctx.DrawText(fmt.Sprintf("chunks: %d  pos: %.1f %.1f %.1f", len(d.chunks), d.player.X, d.player.Y, d.player.Z), 16, 50, 14, color.NRGBA{R: 212, G: 226, B: 235, A: 255})
	return nil
}

func (d *demo) eyePosition() gowin.Vec3 {
	return d.player.Add(gowin.Vec3{Y: eyeHeight})
}

func (d *demo) lookDirection() gowin.Vec3 {
	cp := float32(math.Cos(float64(d.pitch)))
	return gowin.Vec3{
		X: float32(math.Sin(float64(d.yaw))) * cp,
		Y: float32(math.Sin(float64(d.pitch))),
		Z: float32(-math.Cos(float64(d.yaw))) * cp,
	}
}

func (d *demo) updateSelection(ctx *gowin.Context) error {
	hit, ok := d.raycast()
	if !ok {
		d.selection = nil
		d.selectionDraw = nil
		if d.selectionMesh != nil {
			d.selectionMesh.Destroy()
			d.selectionMesh = nil
		}
		return nil
	}
	d.selection = &hit
	data := outlineMeshData(hit.block)
	mesh := ctx.NewMesh(gowin.MeshOptions{Usage: gowin.DynamicMesh})
	if err := mesh.SetData(data); err != nil {
		return err
	}
	if d.selectionMesh != nil {
		d.selectionMesh.Destroy()
	}
	draw, err := ctx.PrepareDraw(mesh, gowin.DrawOptions{
		Uniforms: gowin.Uniforms{"u_Ambient": float32(0.9)},
	})
	if err != nil {
		mesh.Destroy()
		return err
	}
	d.selectionMesh = mesh
	d.selectionDraw = draw
	return nil
}

func (d *demo) raycast() (rayHit, bool) {
	origin := d.eyePosition()
	dir := d.lookDirection().Normalize()
	prev := worldBlock{
		X: int(math.Floor(float64(origin.X))),
		Y: int(math.Floor(float64(origin.Y))),
		Z: int(math.Floor(float64(origin.Z))),
	}
	step := float32(0.04)
	for t := step; t <= reach; t += step {
		p := origin.Add(dir.MulScalar(t))
		cur := worldBlock{
			X: int(math.Floor(float64(p.X))),
			Y: int(math.Floor(float64(p.Y))),
			Z: int(math.Floor(float64(p.Z))),
		}
		if cur == prev {
			continue
		}
		if d.blockAt(cur) != 0 {
			return rayHit{
				block:  cur,
				place:  prev,
				normal: gowin.Vec3{X: float32(prev.X - cur.X), Y: float32(prev.Y - cur.Y), Z: float32(prev.Z - cur.Z)},
			}, true
		}
		prev = cur
	}
	return rayHit{}, false
}

func (d *demo) deleteSelection(ctx *gowin.Context) error {
	if d.selection == nil {
		return nil
	}
	d.setBlock(ctx, d.selection.block, 0)
	return d.updateSelection(ctx)
}

func (d *demo) placeSelection(ctx *gowin.Context) error {
	if d.selection == nil || d.blockAt(d.selection.place) != 0 {
		return nil
	}
	if blockIntersectsPlayer(d.selection.place, d.player) {
		return nil
	}
	d.setBlock(ctx, d.selection.place, blockSand)
	return d.updateSelection(ctx)
}

func blockIntersectsPlayer(block worldBlock, player gowin.Vec3) bool {
	return float32(block.X+1) > player.X-playerRadius &&
		float32(block.X) < player.X+playerRadius &&
		float32(block.Z+1) > player.Z-playerRadius &&
		float32(block.Z) < player.Z+playerRadius &&
		float32(block.Y+1) > player.Y &&
		float32(block.Y) < player.Y+playerHeight
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
			c := d.newChunk(key)
			d.chunks[key] = c
			if err := d.rebuildChunk(ctx, c); err != nil {
				return err
			}
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

func (d *demo) newChunk(key chunkKey) *chunk {
	c := &chunk{key: key, blocks: map[blockKey]blockKind{}}
	for z := 0; z < chunkSize; z++ {
		for x := 0; x < chunkSize; x++ {
			wx := key.X*chunkSize + x
			wz := key.Z*chunkSize + z
			h := terrainHeight(wx, wz)
			for y := 0; y <= h; y++ {
				kind := blockStone
				if y == h {
					kind = blockGrass
				} else if y > h-3 {
					kind = blockDirt
				}
				c.blocks[blockKey{X: x, Y: y, Z: z}] = kind
			}
			if shouldGrowTree(wx, wz, h) {
				for y := h + 1; y <= h+5; y++ {
					c.blocks[blockKey{X: x, Y: y, Z: z}] = blockWood
				}
				for ly := h + 4; ly <= h+7; ly++ {
					for lz := z - 2; lz <= z+2; lz++ {
						for lx := x - 2; lx <= x+2; lx++ {
							if lx < 0 || lx >= chunkSize || lz < 0 || lz >= chunkSize {
								continue
							}
							if abs(lx-x)+abs(lz-z)+max(0, ly-(h+5)) > 4 {
								continue
							}
							c.blocks[blockKey{X: lx, Y: ly, Z: lz}] = blockLeaves
						}
					}
				}
			}
		}
	}
	c.node = d.scene.NewNode()
	c.node.SetDraw(c.draw)
	return c
}

func (d *demo) blockAt(b worldBlock) blockKind {
	key, local := worldToLocal(b)
	c := d.chunks[key]
	if c == nil {
		return 0
	}
	return c.blocks[local]
}

func (d *demo) setBlock(ctx *gowin.Context, b worldBlock, kind blockKind) {
	key, local := worldToLocal(b)
	c := d.chunks[key]
	if c == nil {
		return
	}
	if kind == 0 {
		delete(c.blocks, local)
	} else {
		c.blocks[local] = kind
	}
	d.rebuildChunk(ctx, c)
	for _, n := range boundaryNeighborKeys(key, local) {
		if other := d.chunks[n]; other != nil {
			d.rebuildChunk(ctx, other)
		}
	}
}

func boundaryNeighborKeys(key chunkKey, local blockKey) []chunkKey {
	var out []chunkKey
	if local.X == 0 {
		out = append(out, chunkKey{X: key.X - 1, Z: key.Z})
	}
	if local.X == chunkSize-1 {
		out = append(out, chunkKey{X: key.X + 1, Z: key.Z})
	}
	if local.Z == 0 {
		out = append(out, chunkKey{X: key.X, Z: key.Z - 1})
	}
	if local.Z == chunkSize-1 {
		out = append(out, chunkKey{X: key.X, Z: key.Z + 1})
	}
	return out
}

func (d *demo) rebuildChunk(ctx *gowin.Context, c *chunk) error {
	data := d.buildChunkMesh(c)
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
			"u_Ambient":        float32(0.38),
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

func (d *demo) buildChunkMesh(c *chunk) gowin.MeshData {
	var vertices []gowin.Vertex3D
	var indices []uint32
	for b, kind := range c.blocks {
		world := worldBlock{X: c.key.X*chunkSize + b.X, Y: b.Y, Z: c.key.Z*chunkSize + b.Z}
		for _, face := range cubeFaces {
			n := worldBlock{X: world.X + int(face.normal.X), Y: world.Y + int(face.normal.Y), Z: world.Z + int(face.normal.Z)}
			if d.blockAt(n) != 0 {
				continue
			}
			base := uint32(len(vertices))
			col := blockFaceColor(kind, face.normal, world)
			for _, p := range face.points {
				vertices = append(vertices, gowin.Vertex3D{
					Position: gowin.Vec3{
						X: float32(b.X) + p.X,
						Y: float32(b.Y) + p.Y,
						Z: float32(b.Z) + p.Z,
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

func outlineMeshData(b worldBlock) gowin.MeshData {
	min := gowin.Vec3{X: float32(b.X) - 0.015, Y: float32(b.Y) - 0.015, Z: float32(b.Z) - 0.015}
	max := gowin.Vec3{X: float32(b.X) + 1.015, Y: float32(b.Y) + 1.015, Z: float32(b.Z) + 1.015}
	const t = float32(0.035)
	col := color.NRGBA{R: 255, G: 232, B: 76, A: 255}
	var data gowin.MeshData
	addBox := func(a, z gowin.Vec3) {
		appendBox(&data, a, z, col)
	}
	addBox(gowin.Vec3{X: min.X, Y: min.Y, Z: min.Z}, gowin.Vec3{X: max.X, Y: min.Y + t, Z: min.Z + t})
	addBox(gowin.Vec3{X: min.X, Y: min.Y, Z: max.Z - t}, gowin.Vec3{X: max.X, Y: min.Y + t, Z: max.Z})
	addBox(gowin.Vec3{X: min.X, Y: max.Y - t, Z: min.Z}, gowin.Vec3{X: max.X, Y: max.Y, Z: min.Z + t})
	addBox(gowin.Vec3{X: min.X, Y: max.Y - t, Z: max.Z - t}, gowin.Vec3{X: max.X, Y: max.Y, Z: max.Z})
	addBox(gowin.Vec3{X: min.X, Y: min.Y, Z: min.Z}, gowin.Vec3{X: min.X + t, Y: max.Y, Z: min.Z + t})
	addBox(gowin.Vec3{X: max.X - t, Y: min.Y, Z: min.Z}, gowin.Vec3{X: max.X, Y: max.Y, Z: min.Z + t})
	addBox(gowin.Vec3{X: min.X, Y: min.Y, Z: max.Z - t}, gowin.Vec3{X: min.X + t, Y: max.Y, Z: max.Z})
	addBox(gowin.Vec3{X: max.X - t, Y: min.Y, Z: max.Z - t}, gowin.Vec3{X: max.X, Y: max.Y, Z: max.Z})
	addBox(gowin.Vec3{X: min.X, Y: min.Y, Z: min.Z}, gowin.Vec3{X: min.X + t, Y: min.Y + t, Z: max.Z})
	addBox(gowin.Vec3{X: max.X - t, Y: min.Y, Z: min.Z}, gowin.Vec3{X: max.X, Y: min.Y + t, Z: max.Z})
	addBox(gowin.Vec3{X: min.X, Y: max.Y - t, Z: min.Z}, gowin.Vec3{X: min.X + t, Y: max.Y, Z: max.Z})
	addBox(gowin.Vec3{X: max.X - t, Y: max.Y - t, Z: min.Z}, gowin.Vec3{X: max.X, Y: max.Y, Z: max.Z})
	return data
}

func appendBox(data *gowin.MeshData, min, max gowin.Vec3, col color.Color) {
	points := []gowin.Vec3{
		{X: min.X, Y: min.Y, Z: max.Z}, {X: max.X, Y: min.Y, Z: max.Z}, {X: max.X, Y: max.Y, Z: max.Z}, {X: min.X, Y: max.Y, Z: max.Z},
		{X: max.X, Y: min.Y, Z: min.Z}, {X: min.X, Y: min.Y, Z: min.Z}, {X: min.X, Y: max.Y, Z: min.Z}, {X: max.X, Y: max.Y, Z: min.Z},
	}
	normals := []gowin.Vec3{{Z: 1}, {Z: -1}, {Y: 1}, {Y: -1}, {X: 1}, {X: -1}}
	faces := [][4]int{{0, 1, 2, 3}, {4, 5, 6, 7}, {3, 2, 7, 6}, {5, 4, 1, 0}, {1, 4, 7, 2}, {5, 0, 3, 6}}
	for i, face := range faces {
		base := uint32(len(data.Vertices))
		for _, idx := range face {
			data.Vertices = append(data.Vertices, gowin.Vertex3D{Position: points[idx], Normal: normals[i], Color: col})
		}
		data.Indices = append(data.Indices, base, base+1, base+2, base, base+2, base+3)
	}
}

func terrainHeight(x, z int) int {
	wave := math.Sin(float64(x)*0.18) + math.Cos(float64(z)*0.15) + math.Sin(float64(x+z)*0.065)
	ridge := math.Abs(math.Sin(float64(x-z) * 0.055))
	return 6 + int(math.Round(wave*3.4+ridge*5.2))
}

func shouldGrowTree(x, z, h int) bool {
	return h > 7 && hash3(x, h, z)%43 == 0
}

func blockFaceColor(kind blockKind, normal gowin.Vec3, b worldBlock) color.Color {
	var base color.NRGBA
	switch kind {
	case blockGrass:
		if normal.Y > 0 {
			base = color.NRGBA{R: 78, G: 161, B: 75, A: 255}
		} else {
			base = color.NRGBA{R: 96, G: 112, B: 63, A: 255}
		}
	case blockDirt:
		base = color.NRGBA{R: 116, G: 82, B: 52, A: 255}
	case blockStone:
		base = color.NRGBA{R: 95, G: 99, B: 108, A: 255}
	case blockWood:
		base = color.NRGBA{R: 119, G: 78, B: 44, A: 255}
	case blockLeaves:
		base = color.NRGBA{R: 50, G: 128, B: 64, A: 255}
	case blockSand:
		base = color.NRGBA{R: 210, G: 178, B: 96, A: 255}
	}
	noise := int(hash3(b.X, b.Y, b.Z)%31) - 15
	shade := 0
	if normal.Y < 0 {
		shade = -35
	} else if normal.X != 0 || normal.Z != 0 {
		shade = -18
	}
	return color.NRGBA{
		R: uint8(clampInt(int(base.R)+noise+shade, 0, 255)),
		G: uint8(clampInt(int(base.G)+noise+shade, 0, 255)),
		B: uint8(clampInt(int(base.B)+noise+shade, 0, 255)),
		A: 255,
	}
}

func worldChunk(x, z float32) chunkKey {
	return chunkKey{
		X: floorDiv(int(math.Floor(float64(x))), chunkSize),
		Z: floorDiv(int(math.Floor(float64(z))), chunkSize),
	}
}

func worldToLocal(b worldBlock) (chunkKey, blockKey) {
	key := chunkKey{X: floorDiv(b.X, chunkSize), Z: floorDiv(b.Z, chunkSize)}
	return key, blockKey{X: positiveMod(b.X, chunkSize), Y: b.Y, Z: positiveMod(b.Z, chunkSize)}
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

func clampInt(v, min, max int) int {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func hash3(x, y, z int) uint32 {
	h := uint32(x*374761393 + y*668265263 + z*2246822519)
	h ^= h >> 13
	h *= 1274126177
	return h ^ (h >> 16)
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
