package gowin

import (
	"image/color"
	"time"

	"github.com/tinyrange/gowin/graphics"
	"github.com/tinyrange/gowin/text"
	"github.com/tinyrange/gowin/window"
)

type Config struct {
	Title      string
	Width      int
	Height     int
	ClearColor color.Color
}

type Game interface {
	Init(ctx *Context) error
	Update(ctx *Context, dt float32) error
	Draw(ctx *Context) error
}

type Shutdowner interface {
	Shutdown(ctx *Context) error
}

type GameTime struct {
	Delta   float32
	Elapsed float32
	Frame   uint64
}

type Context struct {
	win      graphics.Window
	frame    graphics.Frame
	text     *text.Renderer
	time     GameTime
	camera3D *Camera3D
}

func Run(game Game, cfg Config) error {
	if cfg.Title == "" {
		cfg.Title = "gowin"
	}
	if cfg.Width == 0 {
		cfg.Width = 800
	}
	if cfg.Height == 0 {
		cfg.Height = 600
	}

	win, err := graphics.New(cfg.Title, cfg.Width, cfg.Height)
	if err != nil {
		return err
	}
	if cfg.ClearColor != nil {
		win.SetClearColor(cfg.ClearColor)
	}

	renderer, err := text.Load(win)
	if err != nil {
		return err
	}

	ctx := &Context{win: win, text: renderer}
	if err := game.Init(ctx); err != nil {
		return err
	}

	start := time.Now()
	last := start
	err = win.Loop(func(f graphics.Frame) error {
		now := time.Now()
		ctx.frame = f
		ctx.time = GameTime{
			Delta:   float32(now.Sub(last).Seconds()),
			Elapsed: float32(now.Sub(start).Seconds()),
			Frame:   ctx.time.Frame + 1,
		}
		last = now

		if err := game.Update(ctx, ctx.time.Delta); err != nil {
			return err
		}
		w, h := f.WindowSize()
		ctx.text.SetViewportScale(int32(w), int32(h), f.Scale())
		return game.Draw(ctx)
	})
	if err != nil {
		return err
	}
	if s, ok := game.(Shutdowner); ok {
		return s.Shutdown(ctx)
	}
	return nil
}

func (c *Context) Time() GameTime {
	return c.time
}

func (c *Context) WindowSize() (int, int) {
	if c.frame == nil {
		return 0, 0
	}
	return c.frame.WindowSize()
}

func (c *Context) MousePosition() Vec2 {
	if c.frame == nil {
		return Vec2{}
	}
	x, y := c.frame.CursorPos()
	return Vec2{X: x, Y: y}
}

func (c *Context) IsKeyDown(key window.Key) bool {
	return c.frame != nil && c.frame.GetKeyState(key).IsDown()
}

func (c *Context) IsKeyPressed(key window.Key) bool {
	return c.frame != nil && c.frame.GetKeyState(key) == window.KeyStatePressed
}

func (c *Context) IsButtonDown(button window.Button) bool {
	return c.frame != nil && c.frame.GetButtonState(button).IsDown()
}

func (c *Context) IsButtonPressed(button window.Button) bool {
	return c.frame != nil && c.frame.GetButtonState(button) == window.ButtonStatePressed
}

func (c *Context) TextInput() string {
	if c.frame == nil {
		return ""
	}
	return c.frame.TextInput()
}

type Key = window.Key
type MouseButton = window.Button

const (
	KeyQ      = window.KeyQ
	KeyW      = window.KeyW
	KeyE      = window.KeyE
	KeyR      = window.KeyR
	KeyA      = window.KeyA
	KeyS      = window.KeyS
	KeyD      = window.KeyD
	KeyF      = window.KeyF
	KeySpace  = window.KeySpace
	KeyEscape = window.KeyEscape

	MouseLeft   = window.ButtonLeft
	MouseRight  = window.ButtonRight
	MouseMiddle = window.ButtonMiddle
)
