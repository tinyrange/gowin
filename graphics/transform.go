package graphics

import "math"

// Mat4 is a column-major 4x4 matrix compatible with OpenGL uniforms.
type Mat4 [16]float32

type Vec3 struct {
	X float32
	Y float32
	Z float32
}

func IdentityMat4() Mat4 {
	return Mat4{
		1, 0, 0, 0,
		0, 1, 0, 0,
		0, 0, 1, 0,
		0, 0, 0, 1,
	}
}

func TranslateMat4(x, y float32) Mat4 {
	m := IdentityMat4()
	m[12] = x
	m[13] = y
	return m
}

func Translate3DMat4(x, y, z float32) Mat4 {
	m := IdentityMat4()
	m[12] = x
	m[13] = y
	m[14] = z
	return m
}

func ScaleMat4(x, y float32) Mat4 {
	m := IdentityMat4()
	m[0] = x
	m[5] = y
	return m
}

func Scale3DMat4(x, y, z float32) Mat4 {
	m := IdentityMat4()
	m[0] = x
	m[5] = y
	m[10] = z
	return m
}

// RotateZMat4 returns a rotation matrix around the Z axis (in radians).
func RotateZMat4(angle float32) Mat4 {
	c := float32(math.Cos(float64(angle)))
	s := float32(math.Sin(float64(angle)))
	// Column-major:
	// [ c -s  0  0 ]
	// [ s  c  0  0 ]
	// [ 0  0  1  0 ]
	// [ 0  0  0  1 ]
	return Mat4{
		c, s, 0, 0,
		-s, c, 0, 0,
		0, 0, 1, 0,
		0, 0, 0, 1,
	}
}

func RotateXMat4(angle float32) Mat4 {
	c := float32(math.Cos(float64(angle)))
	s := float32(math.Sin(float64(angle)))
	return Mat4{
		1, 0, 0, 0,
		0, c, s, 0,
		0, -s, c, 0,
		0, 0, 0, 1,
	}
}

func RotateYMat4(angle float32) Mat4 {
	c := float32(math.Cos(float64(angle)))
	s := float32(math.Sin(float64(angle)))
	return Mat4{
		c, 0, -s, 0,
		0, 1, 0, 0,
		s, 0, c, 0,
		0, 0, 0, 1,
	}
}

// MulMat4 returns a*b (column-major, vectors on the right).
func MulMat4(a, b Mat4) Mat4 {
	var r Mat4
	for c := 0; c < 4; c++ {
		for row := 0; row < 4; row++ {
			r[c*4+row] =
				a[0*4+row]*b[c*4+0] +
					a[1*4+row]*b[c*4+1] +
					a[2*4+row]*b[c*4+2] +
					a[3*4+row]*b[c*4+3]
		}
	}
	return r
}

func PerspectiveMat4(fovYRadians, aspect, near, far float32) Mat4 {
	f := float32(1 / math.Tan(float64(fovYRadians)/2))
	return Mat4{
		f / aspect, 0, 0, 0,
		0, f, 0, 0,
		0, 0, (far + near) / (near - far), -1,
		0, 0, (2 * far * near) / (near - far), 0,
	}
}

func Ortho3DMat4(left, right, bottom, top, near, far float32) Mat4 {
	return Mat4{
		2 / (right - left), 0, 0, 0,
		0, 2 / (top - bottom), 0, 0,
		0, 0, -2 / (far - near), 0,
		-(right + left) / (right - left), -(top + bottom) / (top - bottom), -(far + near) / (far - near), 1,
	}
}

func LookAtMat4(eye, center, up Vec3) Mat4 {
	f := normalize3(sub3(center, eye))
	s := normalize3(cross3(f, up))
	u := cross3(s, f)

	return Mat4{
		s.X, u.X, -f.X, 0,
		s.Y, u.Y, -f.Y, 0,
		s.Z, u.Z, -f.Z, 0,
		-dot3(s, eye), -dot3(u, eye), dot3(f, eye), 1,
	}
}

func normalize3(v Vec3) Vec3 {
	l := float32(math.Sqrt(float64(v.X*v.X + v.Y*v.Y + v.Z*v.Z)))
	if l == 0 {
		return Vec3{}
	}
	return Vec3{X: v.X / l, Y: v.Y / l, Z: v.Z / l}
}

func sub3(a, b Vec3) Vec3 {
	return Vec3{X: a.X - b.X, Y: a.Y - b.Y, Z: a.Z - b.Z}
}

func cross3(a, b Vec3) Vec3 {
	return Vec3{
		X: a.Y*b.Z - a.Z*b.Y,
		Y: a.Z*b.X - a.X*b.Z,
		Z: a.X*b.Y - a.Y*b.X,
	}
}

func dot3(a, b Vec3) float32 {
	return a.X*b.X + a.Y*b.Y + a.Z*b.Z
}
