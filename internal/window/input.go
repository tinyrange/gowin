package window

// Key represents a keyboard key. The set is intentionally small for now and
// can grow as input handling is added.
type Key int

const (
	KeyUnknown Key = iota
)

// Button represents a mouse button.
type Button int

const (
	ButtonLeft Button = iota
	ButtonRight
	ButtonMiddle
)
