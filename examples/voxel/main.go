package main

import (
	"errors"
	"flag"
	"fmt"
	"image/color"
	"math"
	"runtime"
	"sort"
	"sync/atomic"
	"time"

	"github.com/tinyrange/gowin"
)

var errDone = errors.New("voxel demo complete")

const (
	chunkSize    = 16
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
	Y int
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
	blockSnow
	blockCrystal
)

type chunk struct {
	key         chunkKey
	blocks      [chunkSize * chunkSize * chunkSize]blockKind
	mesh        *gowin.Mesh
	draw        *gowin.DrawCommand
	node        *gowin.Node
	dirty       bool
	lod         int
	vertexCount int
	loadDist    int
}

type chunkJob struct {
	key  chunkKey
	lod  int
	dist int
}

type chunkResult struct {
	chunk   *chunk
	elapsed time.Duration
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
	maxFrames       int
	viewRadius      int
	verticalRadius  int
	drawBudget      int
	vertexBudget    int
	scene           *gowin.Scene
	chunks          map[chunkKey]*chunk
	pending         map[chunkKey]bool
	jobs            chan chunkJob
	results         chan chunkResult
	stopWorkers     chan struct{}
	workersStarted  bool
	visibleVertices int
	visibleChunks   int
	pendingCount    int
	completedChunks uint64
	totalGenNanos   int64
	rebuiltMeshes   uint64
	totalMeshNanos  int64
	uploadBudget    int

	player       gowin.Vec3 // feet position
	vel          gowin.Vec3
	yaw          float32
	pitch        float32
	onGround     bool
	flying       bool
	lastSpaceTap float32

	selection     *rayHit
	selectionMesh *gowin.Mesh
	selectionDraw *gowin.DrawCommand
}

func (d *demo) Init(ctx *gowin.Context) error {
	d.scene = gowin.NewScene()
	d.chunks = map[chunkKey]*chunk{}
	d.pending = map[chunkKey]bool{}
	d.jobs = make(chan chunkJob, 4096)
	d.results = make(chan chunkResult, 4096)
	d.stopWorkers = make(chan struct{})
	d.player = findSpawnPoint()
	d.yaw = -0.8
	d.pitch = -0.15
	ctx.SetMouseCaptured(true)
	d.startWorkers()
	return d.streamChunks(ctx)
}

func (d *demo) Shutdown(ctx *gowin.Context) error {
	if d.stopWorkers != nil {
		close(d.stopWorkers)
		d.stopWorkers = nil
	}
	return nil
}

func (d *demo) startWorkers() {
	if d.workersStarted {
		return
	}
	d.workersStarted = true
	n := max(1, runtime.NumCPU()-1)
	for i := 0; i < n; i++ {
		go func() {
			for {
				select {
				case <-d.stopWorkers:
					return
				case job := <-d.jobs:
					start := time.Now()
					c := generateChunk(job.key)
					c.lod = job.lod
					c.loadDist = job.dist
					select {
					case d.results <- chunkResult{chunk: c, elapsed: time.Since(start)}:
					case <-d.stopWorkers:
						return
					}
				}
			}
		}()
	}
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
	if ctx.IsKeyPressed(gowin.KeySpace) {
		now := ctx.Time().Elapsed
		if now-d.lastSpaceTap < 0.32 {
			d.flying = !d.flying
			d.vel = gowin.Vec3{}
			d.onGround = false
			d.lastSpaceTap = 0
		} else {
			d.lastSpaceTap = now
		}
	}
	if d.flying {
		return d.updateFlight(ctx, dt)
	}

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
	if d.player.Y < -64 {
		d.player = findSpawnPoint()
		d.vel = gowin.Vec3{}
	}
	return nil
}

func (d *demo) updateFlight(ctx *gowin.Context, dt float32) error {
	look := d.lookDirection()
	forward := gowin.Vec3{X: look.X, Z: look.Z}.Normalize()
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
	if ctx.IsKeyDown(gowin.KeySpace) {
		wish.Y += 1
	}
	if ctx.IsKeyDown(gowin.KeyCtrl) || ctx.IsKeyDown(gowin.KeyRCtrl) {
		wish.Y -= 1
	}
	speed := sprintSpeed
	if ctx.IsKeyDown(gowin.KeyShift) || ctx.IsKeyDown(gowin.KeyRShift) {
		speed = sprintSpeed * 2.2
	}
	d.player = d.player.Add(wish.Normalize().MulScalar(speed * dt))
	d.vel = gowin.Vec3{}
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

	mode := "walk"
	if d.flying {
		mode = "fly"
	}
	ctx.DrawText("WASD move  double-tap Space fly  Space/Ctrl up/down  Shift sprint  LMB/RMB edit  Esc release", 16, 26, 16, color.White)
	ctx.DrawText(fmt.Sprintf("draws: %d/%d  chunks: %d +%d  verts: %d/%d  gen: %.2fms mesh: %.2fms", d.visibleChunks, d.drawBudget, len(d.chunks), d.pendingCount, d.visibleVertices, d.vertexBudget, d.averageGenMillis(), d.averageMeshMillis()), 16, 50, 14, color.NRGBA{R: 212, G: 226, B: 235, A: 255})
	ctx.DrawText("mode: "+mode, 16, 72, 14, color.NRGBA{R: 232, G: 230, B: 154, A: 255})
	return nil
}

func (d *demo) averageGenMillis() float64 {
	return averageMillis(atomic.LoadInt64(&d.totalGenNanos), atomic.LoadUint64(&d.completedChunks))
}

func (d *demo) averageMeshMillis() float64 {
	return averageMillis(atomic.LoadInt64(&d.totalMeshNanos), atomic.LoadUint64(&d.rebuiltMeshes))
}

func averageMillis(nanos int64, count uint64) float64 {
	if count == 0 {
		return 0
	}
	return float64(nanos) / float64(count) / 1e6
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

func findSpawnPoint() gowin.Vec3 {
	for radius := 0; radius <= 256; radius += 8 {
		for z := -radius; z <= radius; z += 8 {
			for x := -radius; x <= radius; x += 8 {
				if abs(x) != radius && abs(z) != radius {
					continue
				}
				for y := 170; y >= 24; y-- {
					if generatedBlock(worldBlock{X: x, Y: y, Z: z}) == 0 {
						continue
					}
					if generatedBlock(worldBlock{X: x, Y: y + 1, Z: z}) == 0 && generatedBlock(worldBlock{X: x, Y: y + 2, Z: z}) == 0 {
						return gowin.Vec3{X: float32(x) + 0.5, Y: float32(y + 1), Z: float32(z) + 0.5}
					}
				}
			}
		}
	}
	return gowin.Vec3{X: 80.5, Y: 150, Z: 80.5}
}

func (d *demo) streamChunks(ctx *gowin.Context) error {
	d.processChunkResults(ctx)
	center := worldChunk(d.player.X, d.player.Y, d.player.Z)
	if d.viewRadius == 0 {
		d.viewRadius = 16
	}
	if d.verticalRadius == 0 {
		d.verticalRadius = 8
	}
	if d.drawBudget == 0 {
		d.drawBudget = 1000
	}
	if d.vertexBudget == 0 {
		d.vertexBudget = 2200000
	}
	if d.uploadBudget == 0 {
		d.uploadBudget = 10
	}
	needed := map[chunkKey]bool{}
	candidates := make([]chunkCandidate, 0, (d.viewRadius*2+1)*(d.viewRadius*2+1)*(d.verticalRadius*2+1))
	for dy := -d.verticalRadius; dy <= d.verticalRadius; dy++ {
		for dz := -d.viewRadius; dz <= d.viewRadius; dz++ {
			for dx := -d.viewRadius; dx <= d.viewRadius; dx++ {
				dist := max(abs(dx), max(abs(dy), abs(dz)))
				if dist > d.viewRadius {
					continue
				}
				key := chunkKey{X: center.X + dx, Y: center.Y + dy, Z: center.Z + dz}
				candidates = append(candidates, chunkCandidate{key: key, dist: dist, lod: lodForDistance(dist)})
			}
		}
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].dist != candidates[j].dist {
			return candidates[i].dist < candidates[j].dist
		}
		return candidates[i].key.Y < candidates[j].key.Y
	})
	d.visibleVertices = 0
	draws := 0
	for _, candidate := range candidates {
		if draws >= d.drawBudget || d.visibleVertices >= d.vertexBudget {
			break
		}
		key := candidate.key
		needed[key] = true
		c := d.chunks[key]
		if c == nil {
			d.enqueueChunk(candidate)
			continue
		}
		if c.lod != candidate.lod {
			c.lod = candidate.lod
			c.dirty = true
		}
		if c.dirty {
			if err := d.rebuildChunk(ctx, c); err != nil {
				return err
			}
		}
		if c.draw == nil {
			continue
		}
		if d.visibleVertices+c.vertexCount > d.vertexBudget && draws > 0 {
			c.node.SetDraw(nil)
			continue
		}
		c.node.SetDraw(c.draw)
		d.visibleVertices += c.vertexCount
		draws++
	}
	d.visibleChunks = draws
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
	for key := range d.pending {
		if !needed[key] {
			delete(d.pending, key)
		}
	}
	d.pendingCount = len(d.pending)
	return nil
}

func (d *demo) processChunkResults(ctx *gowin.Context) {
	if d.results == nil {
		return
	}
	for i := 0; i < d.uploadBudget; i++ {
		select {
		case result := <-d.results:
			c := result.chunk
			if c == nil || !d.pending[c.key] {
				continue
			}
			delete(d.pending, c.key)
			c.node = d.scene.NewNode()
			d.chunks[c.key] = c
			d.markChunkDirty(c.key)
			for _, n := range allNeighborKeys(c.key) {
				d.markChunkDirty(n)
			}
			_ = d.rebuildChunk(ctx, c)
			atomic.AddUint64(&d.completedChunks, 1)
			atomic.AddInt64(&d.totalGenNanos, result.elapsed.Nanoseconds())
		default:
			return
		}
	}
}

func (d *demo) enqueueChunk(candidate chunkCandidate) {
	if d.jobs == nil || d.pending[candidate.key] {
		return
	}
	d.pending[candidate.key] = true
	select {
	case d.jobs <- chunkJob{key: candidate.key, lod: candidate.lod, dist: candidate.dist}:
	default:
		delete(d.pending, candidate.key)
	}
}

type chunkCandidate struct {
	key  chunkKey
	dist int
	lod  int
}

func lodForDistance(dist int) int {
	switch {
	case dist <= 5:
		return 0
	case dist <= 8:
		return 1
	default:
		return 2
	}
}

func generateChunk(key chunkKey) *chunk {
	c := &chunk{key: key, dirty: true}
	for z := 0; z < chunkSize; z++ {
		for x := 0; x < chunkSize; x++ {
			wx := key.X*chunkSize + x
			wz := key.Z*chunkSize + z
			for y := 0; y < chunkSize; y++ {
				world := worldBlock{
					X: wx,
					Y: key.Y*chunkSize + y,
					Z: wz,
				}
				if kind := generatedBlock(world); kind != 0 {
					c.setLocal(blockKey{X: x, Y: y, Z: z}, kind)
				}
			}
		}
	}
	return c
}

func (c *chunk) blockLocal(b blockKey) blockKind {
	return c.blocks[localIndex(b)]
}

func (c *chunk) setLocal(b blockKey, kind blockKind) {
	c.blocks[localIndex(b)] = kind
}

func localIndex(b blockKey) int {
	return (b.Y*chunkSize+b.Z)*chunkSize + b.X
}

func (d *demo) blockAt(b worldBlock) blockKind {
	key, local := worldToLocal(b)
	c := d.chunks[key]
	if c == nil {
		return 0
	}
	return c.blockLocal(local)
}

func (d *demo) setBlock(ctx *gowin.Context, b worldBlock, kind blockKind) {
	key, local := worldToLocal(b)
	c := d.chunks[key]
	if c == nil {
		return
	}
	if kind == 0 {
		c.setLocal(local, 0)
	} else {
		c.setLocal(local, kind)
	}
	c.dirty = true
	for _, n := range boundaryNeighborKeys(key, local) {
		if other := d.chunks[n]; other != nil {
			other.dirty = true
		}
	}
	_ = d.rebuildChunk(ctx, c)
	for _, n := range boundaryNeighborKeys(key, local) {
		if other := d.chunks[n]; other != nil {
			_ = d.rebuildChunk(ctx, other)
		}
	}
}

func boundaryNeighborKeys(key chunkKey, local blockKey) []chunkKey {
	var out []chunkKey
	if local.X == 0 {
		out = append(out, chunkKey{X: key.X - 1, Y: key.Y, Z: key.Z})
	}
	if local.X == chunkSize-1 {
		out = append(out, chunkKey{X: key.X + 1, Y: key.Y, Z: key.Z})
	}
	if local.Y == 0 {
		out = append(out, chunkKey{X: key.X, Y: key.Y - 1, Z: key.Z})
	}
	if local.Y == chunkSize-1 {
		out = append(out, chunkKey{X: key.X, Y: key.Y + 1, Z: key.Z})
	}
	if local.Z == 0 {
		out = append(out, chunkKey{X: key.X, Y: key.Y, Z: key.Z - 1})
	}
	if local.Z == chunkSize-1 {
		out = append(out, chunkKey{X: key.X, Y: key.Y, Z: key.Z + 1})
	}
	return out
}

func allNeighborKeys(key chunkKey) []chunkKey {
	return []chunkKey{
		{X: key.X - 1, Y: key.Y, Z: key.Z},
		{X: key.X + 1, Y: key.Y, Z: key.Z},
		{X: key.X, Y: key.Y - 1, Z: key.Z},
		{X: key.X, Y: key.Y + 1, Z: key.Z},
		{X: key.X, Y: key.Y, Z: key.Z - 1},
		{X: key.X, Y: key.Y, Z: key.Z + 1},
	}
}

func (d *demo) markChunkDirty(key chunkKey) {
	if c := d.chunks[key]; c != nil {
		c.dirty = true
	}
}

func (d *demo) rebuildChunk(ctx *gowin.Context, c *chunk) error {
	start := time.Now()
	defer func() {
		atomic.AddUint64(&d.rebuiltMeshes, 1)
		atomic.AddInt64(&d.totalMeshNanos, time.Since(start).Nanoseconds())
	}()
	data := d.buildChunkMesh(c)
	if len(data.Vertices) == 0 || len(data.Indices) == 0 {
		if c.mesh != nil {
			c.mesh.Destroy()
			c.mesh = nil
		}
		c.vertexCount = 0
		c.draw = nil
		c.dirty = false
		if c.node != nil {
			c.node.SetDraw(nil)
		}
		return nil
	}
	mesh := ctx.NewMesh(gowin.MeshOptions{Usage: gowin.DynamicMesh})
	if err := mesh.SetData(data); err != nil {
		return err
	}
	if c.mesh != nil {
		c.mesh.Destroy()
	}
	draw, err := ctx.PrepareDraw(mesh, gowin.DrawOptions{
		Transform: gowin.Translate3D(float32(c.key.X*chunkSize), float32(c.key.Y*chunkSize), float32(c.key.Z*chunkSize)),
		Uniforms: gowin.Uniforms{
			"u_Ambient":        float32(0.38),
			"u_LightDirection": gowin.Vec3{X: -0.45, Y: 0.8, Z: 0.35},
			"u_FogStart":       float32(120),
			"u_FogEnd":         float32(420),
			"u_FogColor":       color.NRGBA{R: 92, G: 134, B: 172, A: 245},
		},
	})
	if err != nil {
		mesh.Destroy()
		return err
	}
	c.mesh = mesh
	c.draw = draw
	c.vertexCount = len(data.Vertices)
	c.dirty = false
	if c.node != nil {
		c.node.SetDraw(draw)
	}
	return nil
}

func (d *demo) buildChunkMesh(c *chunk) gowin.MeshData {
	vertices := make([]gowin.Vertex3D, 0, chunkSize*chunkSize*8)
	indices := make([]uint32, 0, chunkSize*chunkSize*12)
	step := 1 << c.lod
	for z := 0; z < chunkSize; z += step {
		for y := 0; y < chunkSize; y += step {
			for x := 0; x < chunkSize; x += step {
				b := blockKey{X: x, Y: y, Z: z}
				kind := c.sampleLOD(b, step)
				if kind == 0 {
					continue
				}
				world := worldBlock{X: c.key.X*chunkSize + b.X, Y: c.key.Y*chunkSize + b.Y, Z: c.key.Z*chunkSize + b.Z}
				for _, face := range cubeFaces {
					n := worldBlock{X: world.X + int(face.normal.X)*step, Y: world.Y + int(face.normal.Y)*step, Z: world.Z + int(face.normal.Z)*step}
					if d.solidVolume(n, step) {
						continue
					}
					base := uint32(len(vertices))
					col := blockFaceColor(kind, face.normal, world)
					for _, p := range face.points {
						vertices = append(vertices, gowin.Vertex3D{
							Position: gowin.Vec3{
								X: float32(b.X) + p.X*float32(step),
								Y: float32(b.Y) + p.Y*float32(step),
								Z: float32(b.Z) + p.Z*float32(step),
							},
							Normal: face.normal,
							UV:     p.UV,
							Color:  col,
						})
					}
					indices = append(indices, base, base+1, base+2, base, base+2, base+3)
				}
			}
		}
	}
	return gowin.MeshData{Vertices: vertices, Indices: indices}
}

func (c *chunk) sampleLOD(start blockKey, step int) blockKind {
	var fallback blockKind
	for z := 0; z < step; z++ {
		for y := 0; y < step; y++ {
			for x := 0; x < step; x++ {
				kind := c.blockLocal(blockKey{X: start.X + x, Y: start.Y + y, Z: start.Z + z})
				if kind == 0 {
					continue
				}
				if kind == blockGrass || kind == blockSnow || kind == blockLeaves || kind == blockCrystal {
					return kind
				}
				fallback = kind
			}
		}
	}
	return fallback
}

func (d *demo) solidVolume(start worldBlock, step int) bool {
	for z := 0; z < step; z++ {
		for y := 0; y < step; y++ {
			for x := 0; x < step; x++ {
				if d.blockAt(worldBlock{X: start.X + x, Y: start.Y + y, Z: start.Z + z}) != 0 {
					return true
				}
			}
		}
	}
	return false
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

func generatedBlock(b worldBlock) blockKind {
	return floatingIslandBlock(b)
}

func crystalPocket(b worldBlock) bool {
	return hash3(b.X, b.Y, b.Z)%29 == 0 && perlinFBM(float64(b.X)*0.09, float64(b.Y)*0.09, float64(b.Z)*0.09, 3) > 0.32
}

func floatingIslandBlock(b worldBlock) blockKind {
	density := floatingIslandDensity(b)
	if density <= 0 {
		return 0
	}
	above := floatingIslandDensity(worldBlock{X: b.X, Y: b.Y + 1, Z: b.Z})
	if above <= 0.04 {
		if b.Y > 100 {
			return blockSnow
		}
		return blockGrass
	}
	if density < 0.2 {
		return blockDirt
	}
	if crystalPocket(b) {
		return blockCrystal
	}
	return blockStone
}

func floatingIslandDensity(b worldBlock) float64 {
	if b.Y < 24 || b.Y > 168 {
		return -1
	}
	best := -2.0
	cellX := floorDiv(b.X, 160)
	cellZ := floorDiv(b.Z, 160)
	for dz := -1; dz <= 1; dz++ {
		for dx := -1; dx <= 1; dx++ {
			cx, cy, cz, rx, ry, rz := islandParams(cellX+dx, cellZ+dz)
			if math.Abs(float64(b.X)-cx) > rx*1.45 || math.Abs(float64(b.Z)-cz) > rz*1.45 || math.Abs(float64(b.Y)-cy) > ry*1.75 {
				continue
			}
			nx := (float64(b.X) - cx) / rx
			ny := (float64(b.Y) - cy) / ry
			nz := (float64(b.Z) - cz) / rz
			shell := 1 - (nx*nx + nz*nz + math.Abs(ny)*ny*1.32)
			warp := perlinFBM((float64(b.X)-cx)*0.025, (float64(b.Y)-cy)*0.033, (float64(b.Z)-cz)*0.025, 3) * 0.58
			carve := perlinFBM(float64(b.X)*0.052+31, float64(b.Y)*0.052-17, float64(b.Z)*0.052+9, 2)
			density := shell + warp
			if carve > 0.48 && density < 0.52 {
				density -= 0.72
			}
			if density > best {
				best = density
			}
		}
	}
	return best
}

func islandParams(cellX, cellZ int) (cx, cy, cz, rx, ry, rz float64) {
	h := hash3(cellX, 97, cellZ)
	cx = float64(cellX*160+80) + float64(int(h&63)-31)
	cz = float64(cellZ*160+80) + float64(int((h>>6)&63)-31)
	cy = 78 + float64(int((h>>12)&63))
	rx = 54 + float64((h>>18)&63)
	rz = 50 + float64((h>>24)&63)
	ry = 30 + float64((h>>10)&31)
	return
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
	case blockSnow:
		base = color.NRGBA{R: 218, G: 229, B: 232, A: 255}
	case blockCrystal:
		base = color.NRGBA{R: 99, G: 220, B: 220, A: 255}
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

func worldChunk(x, y, z float32) chunkKey {
	return chunkKey{
		X: floorDiv(int(math.Floor(float64(x))), chunkSize),
		Y: floorDiv(int(math.Floor(float64(y))), chunkSize),
		Z: floorDiv(int(math.Floor(float64(z))), chunkSize),
	}
}

func worldToLocal(b worldBlock) (chunkKey, blockKey) {
	key := chunkKey{X: floorDiv(b.X, chunkSize), Y: floorDiv(b.Y, chunkSize), Z: floorDiv(b.Z, chunkSize)}
	return key, blockKey{X: positiveMod(b.X, chunkSize), Y: positiveMod(b.Y, chunkSize), Z: positiveMod(b.Z, chunkSize)}
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

func perlinFBM(x, y, z float64, octaves int) float64 {
	var sum, amp, norm float64
	amp = 1
	for i := 0; i < octaves; i++ {
		sum += perlin3(x, y, z) * amp
		norm += amp
		x *= 2.03
		y *= 2.03
		z *= 2.03
		amp *= 0.5
	}
	if norm == 0 {
		return 0
	}
	return sum / norm
}

func perlin3(x, y, z float64) float64 {
	x0 := int(math.Floor(x))
	y0 := int(math.Floor(y))
	z0 := int(math.Floor(z))
	xf := x - float64(x0)
	yf := y - float64(y0)
	zf := z - float64(z0)
	u := fade(xf)
	v := fade(yf)
	w := fade(zf)

	x00 := lerp(grad3(x0, y0, z0, xf, yf, zf), grad3(x0+1, y0, z0, xf-1, yf, zf), u)
	x10 := lerp(grad3(x0, y0+1, z0, xf, yf-1, zf), grad3(x0+1, y0+1, z0, xf-1, yf-1, zf), u)
	x01 := lerp(grad3(x0, y0, z0+1, xf, yf, zf-1), grad3(x0+1, y0, z0+1, xf-1, yf, zf-1), u)
	x11 := lerp(grad3(x0, y0+1, z0+1, xf, yf-1, zf-1), grad3(x0+1, y0+1, z0+1, xf-1, yf-1, zf-1), u)
	y0v := lerp(x00, x10, v)
	y1v := lerp(x01, x11, v)
	return lerp(y0v, y1v, w)
}

func fade(t float64) float64 {
	return t * t * t * (t*(t*6-15) + 10)
}

func lerp(a, b, t float64) float64 {
	return a + (b-a)*t
}

func grad3(ix, iy, iz int, x, y, z float64) float64 {
	h := hash3(ix, iy, iz) & 15
	u := x
	if h >= 8 {
		u = y
	}
	v := y
	if h == 12 || h == 14 {
		v = x
	} else if h >= 4 {
		v = z
	}
	if h&1 != 0 {
		u = -u
	}
	if h&2 != 0 {
		v = -v
	}
	return u + v
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
	view := flag.Int("view", 16, "maximum chunk radius to consider")
	vertical := flag.Int("vertical", 8, "vertical chunk radius to consider")
	drawBudget := flag.Int("draw-budget", 1000, "maximum visible chunk draw commands")
	vertexBudget := flag.Int("vertex-budget", 2200000, "maximum visible mesh vertices")
	flag.Parse()
	err := gowin.Run(&demo{
		maxFrames:      *frames,
		viewRadius:     *view,
		verticalRadius: *vertical,
		drawBudget:     *drawBudget,
		vertexBudget:   *vertexBudget,
	}, gowin.Config{
		Title:      "gowin voxel demo",
		Width:      1100,
		Height:     720,
		ClearColor: color.NRGBA{R: 92, G: 134, B: 172, A: 255},
	})
	if err != nil && !errors.Is(err, errDone) {
		panic(err)
	}
}
