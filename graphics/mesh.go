package graphics

// Rect is a logical-pixel rectangle in the top-left origin coordinate space used
// by Frame drawing methods.
type Rect struct {
	X      float32
	Y      float32
	Width  float32
	Height float32
}

// Point is a logical-pixel coordinate in the top-left origin drawing space.
type Point struct {
	X float32
	Y float32
}

// Vertex matches the graphics shader input layout:
//
//	a_position: vec2
//	a_texCoord: vec2
//	a_color:    vec4
//
// All values are in float32 and packed tightly in this order.
// With a nil mesh texture, vertex colors are multiplied by a built-in white
// texture, so per-vertex colors can be used directly for solid colors and
// tessellated linear/radial gradients.
type Vertex struct {
	X float32
	Y float32
	U float32
	V float32
	R float32
	G float32
	B float32
	A float32
}

// Mesh is an opaque GPU resource created by Window.NewMesh and drawn by Frame.RenderMesh.
type Mesh interface {
	isMesh()
	// Destroy releases the GPU buffers owned by this mesh. Drawing a destroyed
	// mesh is a no-op. Destroy is idempotent.
	Destroy()
}

type DrawOptions struct {
	// Model is a column-major 4x4 model transform applied to vertex positions
	// before projection. If left as the zero value, Identity is assumed.
	Model Mat4
	// Mask multiplies rendered alpha by a mask texture sampled with the mesh UVs.
	// A render target texture works well for SVG-like masks.
	Mask Texture
}

// Vertex3D matches the built-in 3D shader input layout:
//
//	a_position: vec3
//	a_normal:   vec3
//	a_texCoord: vec2
//	a_color:    vec4
//
// Positions are in caller-defined 3D units. The coordinate system is
// right-handed by convention: +X right, +Y up, and +Z toward the camera for the
// default LookAt-style cameras. Front faces use counter-clockwise winding.
type Vertex3D struct {
	X  float32
	Y  float32
	Z  float32
	NX float32
	NY float32
	NZ float32
	U  float32
	V  float32
	R  float32
	G  float32
	B  float32
	A  float32
}

// Mesh3D is an opaque GPU resource created by Window.NewMesh3D and drawn by
// Frame.RenderMesh3D.
type Mesh3D interface {
	isMesh3D()
	// Destroy releases the GPU buffers owned by this mesh. Drawing a destroyed
	// mesh is a no-op. Destroy is idempotent.
	Destroy()
}

// Shader3D is an opaque shader resource created by Window.NewShader3D.
type Shader3D interface {
	isShader3D()
	// Destroy releases the shader program. Destroy is idempotent.
	Destroy()
}

type Draw3DOptions struct {
	// Model places the mesh in world space. Zero value means identity.
	Model Mat4
	// View transforms world coordinates into camera space.
	View Mat4
	// Projection maps camera coordinates to clip space. Use PerspectiveMat4 or
	// Ortho3DMat4 for the common cases.
	Projection Mat4
	// Shader overrides the built-in lit vertex-color shader. Custom shaders must
	// use the Mesh3D vertex layout documented on Window.NewShader3D.
	Shader Shader3D
	// Ambient controls minimum light contribution. Zero uses a small default.
	Ambient float32
	// LightDirection points from the surface toward the directional light.
	// Zero uses a camera/front-left default.
	LightDirection Vec3
	// ClipDepthTest controls whether PushClipMesh3D depth-tests while writing
	// the stencil clip. The zero value is false, which is the recommended
	// projected-outline mode for flat surface clipping.
	ClipDepthTest bool
}
