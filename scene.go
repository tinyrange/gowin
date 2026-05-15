package gowin

type Scene struct {
	nodes []*Node
}

type Node struct {
	Transform Mat4
	Draw      *DrawCommand
	Children  []*Node
}

func NewScene() *Scene {
	return &Scene{}
}

func (s *Scene) NewNode() *Node {
	n := &Node{Transform: Identity()}
	s.nodes = append(s.nodes, n)
	return n
}

func (n *Node) NewChild() *Node {
	child := &Node{Transform: Identity()}
	n.Children = append(n.Children, child)
	return child
}

func (n *Node) SetTransform(transform Mat4) {
	n.Transform = transform
}

func (n *Node) SetDraw(cmd *DrawCommand) {
	n.Draw = cmd
}

func (s *Scene) Draw(ctx *Context) {
	for _, n := range s.nodes {
		drawNode(ctx, n, Identity())
	}
}

func drawNode(ctx *Context, n *Node, parent Mat4) {
	if n == nil {
		return
	}
	world := Mul(parent, n.Transform)
	if n.Draw != nil {
		prev := n.Draw.transform
		n.Draw.transform = world
		ctx.Draw(n.Draw)
		n.Draw.transform = prev
	}
	for _, child := range n.Children {
		drawNode(ctx, child, world)
	}
}
