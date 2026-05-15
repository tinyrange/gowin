package graphics

import "image/color"

// Cuboid3DGeometry returns a colored box centered on the origin. It is useful
// for slabs, blocks, and simple technical preview fixtures. Faces are emitted
// with counter-clockwise winding when viewed from outside.
func Cuboid3DGeometry(width, height, depth float32, c color.Color) ([]Vertex3D, []uint32) {
	hx, hy, hz := width/2, height/2, depth/2
	rgba := ColorToFloat32(c)
	v := func(x, y, z, nx, ny, nz float32) Vertex3D {
		return Vertex3D{
			X: x, Y: y, Z: z,
			NX: nx, NY: ny, NZ: nz,
			R: rgba[0], G: rgba[1], B: rgba[2], A: rgba[3],
		}
	}

	verts := []Vertex3D{
		v(-hx, -hy, hz, 0, 0, 1), v(hx, -hy, hz, 0, 0, 1), v(hx, hy, hz, 0, 0, 1), v(-hx, hy, hz, 0, 0, 1),
		v(hx, -hy, -hz, 0, 0, -1), v(-hx, -hy, -hz, 0, 0, -1), v(-hx, hy, -hz, 0, 0, -1), v(hx, hy, -hz, 0, 0, -1),
		v(-hx, hy, hz, 0, 1, 0), v(hx, hy, hz, 0, 1, 0), v(hx, hy, -hz, 0, 1, 0), v(-hx, hy, -hz, 0, 1, 0),
		v(-hx, -hy, -hz, 0, -1, 0), v(hx, -hy, -hz, 0, -1, 0), v(hx, -hy, hz, 0, -1, 0), v(-hx, -hy, hz, 0, -1, 0),
		v(hx, -hy, hz, 1, 0, 0), v(hx, -hy, -hz, 1, 0, 0), v(hx, hy, -hz, 1, 0, 0), v(hx, hy, hz, 1, 0, 0),
		v(-hx, -hy, -hz, -1, 0, 0), v(-hx, -hy, hz, -1, 0, 0), v(-hx, hy, hz, -1, 0, 0), v(-hx, hy, -hz, -1, 0, 0),
	}
	idx := make([]uint32, 0, 36)
	for face := uint32(0); face < 6; face++ {
		base := face * 4
		idx = append(idx, base, base+1, base+2, base, base+2, base+3)
	}
	return verts, idx
}

// PlanarRect3DGeometry returns a colored rectangle in the XY plane at z. It is
// useful for surface layers and stencil clip meshes projected onto flat faces.
func PlanarRect3DGeometry(width, height, z float32, c color.Color) ([]Vertex3D, []uint32) {
	hx, hy := width/2, height/2
	rgba := ColorToFloat32(c)
	verts := []Vertex3D{
		{X: -hx, Y: -hy, Z: z, NX: 0, NY: 0, NZ: 1, R: rgba[0], G: rgba[1], B: rgba[2], A: rgba[3]},
		{X: hx, Y: -hy, Z: z, NX: 0, NY: 0, NZ: 1, R: rgba[0], G: rgba[1], B: rgba[2], A: rgba[3]},
		{X: hx, Y: hy, Z: z, NX: 0, NY: 0, NZ: 1, R: rgba[0], G: rgba[1], B: rgba[2], A: rgba[3]},
		{X: -hx, Y: hy, Z: z, NX: 0, NY: 0, NZ: 1, R: rgba[0], G: rgba[1], B: rgba[2], A: rgba[3]},
	}
	return verts, []uint32{0, 1, 2, 0, 2, 3}
}

// PlanarPolygon3DGeometry triangulates one planar XY contour at z using a
// triangle fan. It is intended for simple convex clip meshes and surface
// patches. For concave outlines or outlines with holes, tessellate externally
// and upload the result with Window.NewMesh3D.
func PlanarPolygon3DGeometry(points []Point, z float32, c color.Color) ([]Vertex3D, []uint32) {
	if len(points) < 3 {
		return nil, nil
	}
	rgba := ColorToFloat32(c)
	verts := make([]Vertex3D, len(points))
	for i, p := range points {
		verts[i] = Vertex3D{
			X: p.X, Y: p.Y, Z: z,
			NX: 0, NY: 0, NZ: 1,
			R: rgba[0], G: rgba[1], B: rgba[2], A: rgba[3],
		}
	}
	idx := make([]uint32, 0, (len(points)-2)*3)
	for i := 1; i < len(points)-1; i++ {
		idx = append(idx, 0, uint32(i), uint32(i+1))
	}
	return verts, idx
}
