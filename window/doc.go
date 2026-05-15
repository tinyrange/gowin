// Package window provides platform-neutral window, input, and OpenGL context
// primitives.
//
// The input layer is intentionally command-free. It reports keys, text,
// modifiers, mouse buttons, cursor movement, scroll wheels, high-resolution
// scroll deltas, and pinch gestures where the platform exposes them. UI and app
// layers decide whether those generic events mean pan, zoom, edit, select, or
// any other command.
package window
