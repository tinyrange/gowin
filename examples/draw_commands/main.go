package main

import (
	"errors"
	"math"

	"github.com/tinyrange/gowin"
)

var errDone = errors.New("example complete")

type demo struct {
	mesh   *gowin.Mesh
	draw   *gowin.DrawCommand
	scene  *gowin.Scene
	camera gowin.Camera3D
	angle  float32
}

func (d *demo) Init(ctx *gowin.Context) error {
	d.camera = gowin.Camera3D{
		Position: gowin.Vec3{X: 0, Y: 1.4, Z: 5},
		Target:   gowin.Vec3{},
		Up:       gowin.Vec3{Y: 1},
		FOVY:     float32(math.Pi / 4),
	}

	mesh := ctx.NewMesh(gowin.MeshOptions{Usage: gowin.StaticMesh})
	if err := mesh.SetData(cubeMeshData(gowin.Color{R: 0.3, G: 0.55, B: 1, A: 1})); err != nil {
		return err
	}
	d.mesh = mesh

	draw, err := ctx.PrepareDraw(mesh, gowin.DrawOptions{
		Shader: gowin.DefaultShader3D(),
		Uniforms: gowin.Uniforms{
			"u_Ambient":        float32(0.36),
			"u_LightDirection": gowin.Vec3{X: -0.4, Y: 0.7, Z: 0.5},
		},
	})
	if err != nil {
		return err
	}
	d.draw = draw

	d.scene = gowin.NewScene()
	left := d.scene.NewNode()
	left.SetTransform(gowin.Translate3D(-1.2, 0, 0))
	left.SetDraw(draw)

	right := d.scene.NewNode()
	right.SetTransform(gowin.Translate3D(1.2, 0, 0))
	right.SetDraw(draw)

	return nil
}

func (d *demo) Update(ctx *gowin.Context, dt float32) error {
	d.angle += dt
	if ctx.Time().Frame > 180 {
		return errDone
	}
	return nil
}

func (d *demo) Draw(ctx *gowin.Context) error {
	ctx.Begin3D(d.camera)
	d.draw.SetUniform("u_Ambient", float32(0.25+0.15*math.Sin(float64(d.angle))))
	d.scene.Draw(ctx)
	ctx.End3D()

	ctx.DrawText("gowin draw commands", 18, 28, 18, gowin.White)
	return nil
}

func main() {
	err := gowin.Run(&demo{}, gowin.Config{
		Title:      "gowin draw commands",
		Width:      900,
		Height:     600,
		ClearColor: gowin.Color{R: 0.07, G: 0.08, B: 0.1, A: 1},
	})
	if err != nil && !errors.Is(err, errDone) {
		panic(err)
	}
}

func cubeMeshData(c gowin.Color) gowin.MeshData {
	p := []gowin.Vec3{
		{X: -0.5, Y: -0.5, Z: 0.5},
		{X: 0.5, Y: -0.5, Z: 0.5},
		{X: 0.5, Y: 0.5, Z: 0.5},
		{X: -0.5, Y: 0.5, Z: 0.5},
		{X: 0.5, Y: -0.5, Z: -0.5},
		{X: -0.5, Y: -0.5, Z: -0.5},
		{X: -0.5, Y: 0.5, Z: -0.5},
		{X: 0.5, Y: 0.5, Z: -0.5},
	}
	n := []gowin.Vec3{
		{Z: 1}, {Z: -1}, {Y: 1}, {Y: -1}, {X: 1}, {X: -1},
	}
	faces := [][4]int{
		{0, 1, 2, 3},
		{4, 5, 6, 7},
		{3, 2, 7, 6},
		{5, 4, 1, 0},
		{1, 4, 7, 2},
		{5, 0, 3, 6},
	}

	var vertices []gowin.Vertex3D
	var indices []uint32
	for i, face := range faces {
		base := uint32(len(vertices))
		for _, idx := range face {
			vertices = append(vertices, gowin.Vertex3D{
				Position: p[idx],
				Normal:   n[i],
				Color:    c,
			})
		}
		indices = append(indices, base, base+1, base+2, base, base+2, base+3)
	}
	return gowin.MeshData{Vertices: vertices, Indices: indices}
}
