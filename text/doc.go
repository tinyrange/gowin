// Package text provides the supported text rendering path for graphics.Window.
//
// Create a renderer with Load after constructing a graphics.Window, call
// SetViewportScale each frame with the current Frame.WindowSize and Frame.Scale,
// then draw text with RenderText or the BeginBatch/AddText/EndBatch batch API:
//
//	r, err := text.Load(win)
//	if err != nil {
//		return err
//	}
//	err = win.Loop(func(f graphics.Frame) error {
//		w, h := f.WindowSize()
//		r.SetViewportScale(int32(w), int32(h), f.Scale())
//		r.RenderText("hello", 16, 32, 18, graphics.ColorWhite)
//		return nil
//	})
//
// SetViewport also refreshes scale from the graphics.Window passed to Load, so
// existing callers continue to follow monitor scale changes when called inside
// the frame loop. SetViewportScale is useful when code already has a Frame and
// wants the scale contract to be explicit.
//
// The renderer shares the window OpenGL context and restores the graphics shader
// it was given by graphics.Window.GetShaderProgram, so it can be mixed with
// Frame.RenderQuad and Frame.RenderMesh calls in the same frame.
//
// For rotated or skewed text, use RenderTextOptions or
// RenderGradientTextOptions with a graphics.Mat4 model transform.
package text
