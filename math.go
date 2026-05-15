package gowin

import (
	"math"

	"github.com/tinyrange/gowin/graphics"
)

type Vec2 struct {
	X float32
	Y float32
}

type Vec3 struct {
	X float32
	Y float32
	Z float32
}

type Mat4 [16]float32

func (v Vec2) Add(o Vec2) Vec2 {
	return Vec2{X: v.X + o.X, Y: v.Y + o.Y}
}

func (v Vec2) Sub(o Vec2) Vec2 {
	return Vec2{X: v.X - o.X, Y: v.Y - o.Y}
}

func (v Vec2) MulScalar(s float32) Vec2 {
	return Vec2{X: v.X * s, Y: v.Y * s}
}

func Identity() Mat4 {
	return Mat4(graphics.IdentityMat4())
}

func Translate2D(x, y float32) Mat4 {
	return Mat4(graphics.TranslateMat4(x, y))
}

func Translate3D(x, y, z float32) Mat4 {
	return Mat4(graphics.Translate3DMat4(x, y, z))
}

func Scale2D(x, y float32) Mat4 {
	return Mat4(graphics.ScaleMat4(x, y))
}

func Scale3D(x, y, z float32) Mat4 {
	return Mat4(graphics.Scale3DMat4(x, y, z))
}

func RotateX(angleRadians float32) Mat4 {
	return Mat4(graphics.RotateXMat4(angleRadians))
}

func RotateY(angleRadians float32) Mat4 {
	return Mat4(graphics.RotateYMat4(angleRadians))
}

func RotateZ(angleRadians float32) Mat4 {
	return Mat4(graphics.RotateZMat4(angleRadians))
}

func Mul(a, b Mat4) Mat4 {
	return Mat4(graphics.MulMat4(graphics.Mat4(a), graphics.Mat4(b)))
}

func Perspective(fovYRadians, aspect, near, far float32) Mat4 {
	return Mat4(graphics.PerspectiveMat4(fovYRadians, aspect, near, far))
}

func LookAt(eye, center, up Vec3) Mat4 {
	return Mat4(graphics.LookAtMat4(eye.graphics(), center.graphics(), up.graphics()))
}

func (v Vec3) Add(o Vec3) Vec3 {
	return Vec3{X: v.X + o.X, Y: v.Y + o.Y, Z: v.Z + o.Z}
}

func (v Vec3) Sub(o Vec3) Vec3 {
	return Vec3{X: v.X - o.X, Y: v.Y - o.Y, Z: v.Z - o.Z}
}

func (v Vec3) MulScalar(s float32) Vec3 {
	return Vec3{X: v.X * s, Y: v.Y * s, Z: v.Z * s}
}

func (v Vec3) Normalize() Vec3 {
	l := float32(math.Sqrt(float64(v.X*v.X + v.Y*v.Y + v.Z*v.Z)))
	if l == 0 {
		return Vec3{}
	}
	return Vec3{X: v.X / l, Y: v.Y / l, Z: v.Z / l}
}

func (v Vec3) graphics() graphics.Vec3 {
	return graphics.Vec3{X: v.X, Y: v.Y, Z: v.Z}
}

type Camera3D struct {
	Position Vec3
	Target   Vec3
	Up       Vec3
	FOVY     float32
	Near     float32
	Far      float32
}

func (c Camera3D) view() graphics.Mat4 {
	up := c.Up
	if up == (Vec3{}) {
		up = Vec3{Y: 1}
	}
	return graphics.LookAtMat4(c.Position.graphics(), c.Target.graphics(), up.graphics())
}

func (c Camera3D) projection(aspect float32) graphics.Mat4 {
	fovy := c.FOVY
	if fovy == 0 {
		fovy = float32(math.Pi / 4)
	}
	near := c.Near
	if near == 0 {
		near = 0.01
	}
	far := c.Far
	if far == 0 {
		far = 1000
	}
	if aspect == 0 {
		aspect = 1
	}
	return graphics.PerspectiveMat4(fovy, aspect, near, far)
}

type Camera2D struct {
	Position Vec2
	Zoom     float32
	Rotation float32
}
