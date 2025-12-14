package main

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"log"
	"net"
	"os"
	"sync"
	"time"

	"github.com/tinyrange/gowin/internal/graphics"
	"github.com/tinyrange/gowin/internal/rfb"
	"github.com/tinyrange/gowin/internal/text"
	"github.com/tinyrange/gowin/internal/window"
)

type vncClient struct {
	gfx           graphics.Window
	font          *text.Renderer
	rfbConn       *rfb.Connection
	framebuffer   *image.RGBA
	fbMutex       sync.RWMutex
	connecting    bool
	connectError  error
	progress      float32
	serverName    string
	width         int
	height        int
	fbTexture     graphics.Texture
	textureDirty  bool
	windowResized bool
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s <host:port>\n", os.Args[0])
		os.Exit(1)
	}

	addr := os.Args[1]

	// Parse host:port
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		log.Fatalf("Invalid address format: %v", err)
	}
	if host == "" {
		host = "localhost"
	}

	// Create window
	gfx, err := graphics.New("VNC Client", 1024, 768)
	if err != nil {
		log.Fatalf("Failed to create window: %v", err)
	}

	gfx.SetClear(true)
	gfx.SetClearColor(color.RGBA{R: 20, G: 20, B: 20, A: 255})

	// Load font
	font, err := text.Load(gfx)
	if err != nil {
		log.Fatalf("Failed to load font: %v", err)
	}

	client := &vncClient{
		gfx:        gfx,
		font:       font,
		connecting: true,
		progress:   0.0,
	}

	// Start connection in goroutine
	go client.connect(host, port)

	// Run main loop
	err = gfx.Loop(client.frame)
	if err != nil {
		log.Fatalf("Loop error: %v", err)
	}
}

func (c *vncClient) connect(host, port string) {
	// Connect to VNC server with progress tracking
	addr := net.JoinHostPort(host, port)

	// Track TCP connection progress
	progressChan := make(chan float32, 10)
	progressDone := make(chan struct{})
	progressStopped := make(chan struct{})

	go func() {
		defer close(progressStopped)
		// Simulate progress during TCP connection
		for i := 0; i < 100; i++ {
			// Check if we should stop before sleeping
			select {
			case <-progressDone:
				return
			default:
			}

			time.Sleep(10 * time.Millisecond)

			// Check again after sleep, and send if not done
			select {
			case <-progressDone:
				return
			case progressChan <- float32(i) / 100.0:
			}
		}
	}()

	// Start connection attempt
	connChan := make(chan net.Conn, 1)
	errChan := make(chan error, 1)
	go func() {
		conn, err := net.Dial("tcp", addr)
		if err != nil {
			errChan <- err
			return
		}
		connChan <- conn
	}()

	// Update progress from channel
	go func() {
		for p := range progressChan {
			c.progress = p
		}
	}()

	// Wait for connection or error
	select {
	case conn := <-connChan:
		// Connection established, stop progress simulation
		close(progressDone)
		<-progressStopped // Wait for progress goroutine to finish
		c.progress = 1.0
		close(progressChan)

		// Create RFB connection
		rfbConn, err := rfb.NewConn(conn)
		if err != nil {
			conn.Close()
			c.connectError = fmt.Errorf("failed to initialize RFB: %v", err)
			c.connecting = false
			return
		}

		c.rfbConn = rfbConn
		c.connecting = false

		// Process RFB events
		go c.processRFBEvents()
	case err := <-errChan:
		close(progressDone)
		<-progressStopped // Wait for progress goroutine to finish
		close(progressChan)
		c.connectError = fmt.Errorf("failed to connect: %v", err)
		c.connecting = false
		return
	}
}

func (c *vncClient) processRFBEvents() {
	for evt := range c.rfbConn.Events {
		switch e := evt.(type) {
		case *rfb.ConnectedEvent:
			c.serverName = e.Name
			c.width = int(e.FrameBufferWidth)
			c.height = int(e.FrameBufferHeight)
			// Create framebuffer
			c.fbMutex.Lock()
			c.framebuffer = image.NewRGBA(image.Rect(0, 0, c.width, c.height))
			c.textureDirty = true
			c.windowResized = true
			c.fbMutex.Unlock()
			// Request initial update
			if err := c.rfbConn.RequestUpdate(false); err != nil {
				log.Printf("Failed to request update: %v", err)
			}

		case *rfb.UpdateRectangleEvent:
			c.fbMutex.Lock()
			if c.framebuffer != nil {
				bounds := e.Bounds()
				img := e.Image

				if e.BGRA {
					// Convert BGRA to RGBA by swapping R and B bytes
					// The RFB server sends pixels in BGRA format (B=byte0, G=byte1, R=byte2, A=byte3)
					// but image.RGBA expects RGBA format (R=byte0, G=byte1, B=byte2, A=byte3)
					// So we need to swap bytes 0 and 2
					if rgba, ok := img.(*image.RGBA); ok {
						// Swap bytes directly in the pixel array for efficiency
						rectWidth := bounds.Dx()
						rectHeight := bounds.Dy()
						for y := 0; y < rectHeight; y++ {
							rowStart := (bounds.Min.Y + y - rgba.Rect.Min.Y) * rgba.Stride
							colStart := (bounds.Min.X - rgba.Rect.Min.X) * 4
							startIdx := rowStart + colStart
							for x := 0; x < rectWidth; x++ {
								idx := startIdx + x*4
								if idx+3 < len(rgba.Pix) {
									// Swap B (idx+0) and R (idx+2) to convert BGRA -> RGBA
									rgba.Pix[idx+0], rgba.Pix[idx+2] = rgba.Pix[idx+2], rgba.Pix[idx+0]
								}
							}
						}
					}
					// Now draw the corrected image
					draw.Draw(c.framebuffer, bounds, img, bounds.Min, draw.Src)
				} else {
					draw.Draw(c.framebuffer, bounds, img, bounds.Min, draw.Src)
				}
				c.textureDirty = true
			}
			c.fbMutex.Unlock()
			// Request incremental update
			if err := c.rfbConn.RequestUpdate(true); err != nil {
				log.Printf("Failed to request update: %v", err)
			}

		case *rfb.ErrorEvent:
			c.connectError = e
			log.Printf("RFB error: %v", e)
		}
	}
}

func (c *vncClient) frame(f graphics.Frame) error {
	w, h := f.WindowSize()
	c.font.SetViewport(int32(w), int32(h))

	// Handle window resize if framebuffer size is known
	c.fbMutex.RLock()
	needsResize := c.windowResized && c.framebuffer != nil
	c.fbMutex.RUnlock()

	if needsResize {
		// Note: The window API doesn't support programmatic resizing,
		// so we scale the content to fit. The VNC framebuffer will be
		// rendered scaled to fit the current window size.
		c.fbMutex.Lock()
		c.windowResized = false
		c.fbMutex.Unlock()
	}

	if c.connecting {
		c.renderLoading(f, w, h)
		return nil
	}

	if c.connectError != nil {
		c.renderError(f, w, h)
		return nil
	}

	if c.framebuffer == nil {
		c.renderWaiting(f, w, h)
		return nil
	}

	// Render VNC framebuffer
	c.renderVNC(f, w, h)

	// Handle input
	c.handleInput(f)

	return nil
}

func (c *vncClient) renderLoading(f graphics.Frame, w, h int) {
	// Draw progress bar
	barWidth := float32(w) * 0.6
	barHeight := float32(40)
	barX := (float32(w) - barWidth) / 2
	barY := float32(h)/2 - barHeight/2

	// Background
	bgTex := c.createSolidColorTexture(graphics.ColorDarkGray)
	f.RenderQuad(barX, barY, barWidth, barHeight, bgTex, graphics.ColorWhite)

	// Progress fill
	progressWidth := barWidth * c.progress
	if progressWidth > 0 {
		progressTex := c.createSolidColorTexture(graphics.ColorBlue)
		f.RenderQuad(barX, barY, progressWidth, barHeight, progressTex, graphics.ColorWhite)
	}

	// Border
	borderTex := c.createSolidColorTexture(graphics.ColorWhite)
	f.RenderQuad(barX, barY, barWidth, 2, borderTex, graphics.ColorWhite)
	f.RenderQuad(barX, barY+barHeight-2, barWidth, 2, borderTex, graphics.ColorWhite)
	f.RenderQuad(barX, barY, 2, barHeight, borderTex, graphics.ColorWhite)
	f.RenderQuad(barX+barWidth-2, barY, 2, barHeight, borderTex, graphics.ColorWhite)

	// Text
	text := fmt.Sprintf("Connecting... %.0f%%", c.progress*100)
	c.font.RenderText(text, barX, barY-30, 20, graphics.ColorWhite)
}

func (c *vncClient) renderError(f graphics.Frame, w, h int) {
	errorText := fmt.Sprintf("Error: %v", c.connectError)
	c.font.RenderText(errorText, float32(w)/2-200, float32(h)/2, 24, graphics.ColorRed)
}

func (c *vncClient) renderWaiting(f graphics.Frame, w, h int) {
	text := "Waiting for server..."
	c.font.RenderText(text, float32(w)/2-150, float32(h)/2, 24, graphics.ColorWhite)
}

func (c *vncClient) renderVNC(f graphics.Frame, w, h int) {
	c.fbMutex.RLock()
	fb := c.framebuffer
	dirty := c.textureDirty
	c.fbMutex.RUnlock()

	if fb == nil {
		return
	}

	// Update texture if framebuffer changed
	if dirty || c.fbTexture == nil {
		c.fbMutex.Lock()
		tex, err := c.gfx.NewTexture(fb)
		if err != nil {
			c.fbMutex.Unlock()
			log.Printf("Failed to create texture: %v", err)
			return
		}
		c.fbTexture = tex
		c.textureDirty = false
		c.fbMutex.Unlock()
	}

	tex := c.fbTexture

	// Get window scale factor - WindowSize() returns physical pixels,
	// but the coordinate system is in logical pixels (scaled)
	windowScale := c.gfx.Scale()

	// Calculate scaling to fit window while maintaining aspect ratio
	// Note: The window API doesn't support programmatic resizing, so we scale
	// the VNC framebuffer to fit the current window size. The content will be
	// centered and scaled proportionally.
	fbWidth := float32(fb.Bounds().Dx())
	fbHeight := float32(fb.Bounds().Dy())
	// Convert physical pixels to logical pixels for coordinate system
	winWidth := float32(w) / windowScale
	winHeight := float32(h) / windowScale

	scaleX := winWidth / fbWidth
	scaleY := winHeight / fbHeight
	scale := scaleX
	if scaleY < scaleX {
		scale = scaleY
	}

	scaledWidth := fbWidth * scale
	scaledHeight := fbHeight * scale
	x := (winWidth - scaledWidth) / 2
	y := (winHeight - scaledHeight) / 2

	f.RenderQuad(x, y, scaledWidth, scaledHeight, tex, graphics.ColorWhite)
}

func (c *vncClient) handleInput(f graphics.Frame) {
	if c.rfbConn == nil {
		return
	}

	w, h := f.WindowSize()
	mouseX, mouseY := f.CursorPos()

	// Convert window coordinates to VNC coordinates
	c.fbMutex.RLock()
	fb := c.framebuffer
	c.fbMutex.RUnlock()

	if fb == nil {
		return
	}

	// Get window scale factor - WindowSize() returns physical pixels,
	// but CursorPos() returns logical pixels (already scaled)
	windowScale := c.gfx.Scale()

	fbWidth := float32(fb.Bounds().Dx())
	fbHeight := float32(fb.Bounds().Dy())
	// Convert physical pixels to logical pixels for coordinate system
	winWidth := float32(w) / windowScale
	winHeight := float32(h) / windowScale

	scaleX := winWidth / fbWidth
	scaleY := winHeight / fbHeight
	scale := scaleX
	if scaleY < scaleX {
		scale = scaleY
	}

	scaledWidth := fbWidth * scale
	scaledHeight := fbHeight * scale
	offsetX := (winWidth - scaledWidth) / 2
	offsetY := (winHeight - scaledHeight) / 2

	// Check if mouse is over VNC area
	if mouseX < offsetX || mouseX > offsetX+scaledWidth ||
		mouseY < offsetY || mouseY > offsetY+scaledHeight {
		return
	}

	// Convert to VNC coordinates
	vncX := uint16((mouseX - offsetX) / scale)
	vncY := uint16((mouseY - offsetY) / scale)

	// Handle mouse buttons
	var buttons rfb.Buttons
	if f.GetButtonState(window.ButtonLeft).IsDown() {
		buttons.Set(rfb.ButtonLeft)
	}
	if f.GetButtonState(window.ButtonRight).IsDown() {
		buttons.Set(rfb.ButtonRight)
	}
	if f.GetButtonState(window.ButtonMiddle).IsDown() {
		buttons.Set(rfb.ButtonMiddle)
	}

	if err := c.rfbConn.SendPointerEvent(buttons, vncX, vncY); err != nil {
		log.Printf("Failed to send pointer event: %v", err)
	}

	// Handle keyboard - send key events for pressed keys
	// This is a simplified version - in a real implementation you'd track key states
	c.handleKeyboard(f)
}

func (c *vncClient) handleKeyboard(f graphics.Frame) {
	// Map window keys to X11 keysym values (simplified)
	keyMap := map[window.Key]uint32{
		window.KeySpace:     0x0020,
		window.KeyEnter:     0xFF0D,
		window.KeyEscape:    0xFF1B,
		window.KeyBackspace: 0xFF08,
		window.KeyTab:       0xFF09,
		window.KeyUp:        0xFF52,
		window.KeyDown:      0xFF54,
		window.KeyLeft:      0xFF51,
		window.KeyRight:     0xFF53,
		window.KeyF1:        0xFFBE,
		window.KeyF2:        0xFFBF,
		window.KeyF3:        0xFFC0,
		window.KeyF4:        0xFFC1,
		window.KeyF5:        0xFFC2,
		window.KeyF6:        0xFFC3,
		window.KeyF7:        0xFFC4,
		window.KeyF8:        0xFFC5,
		window.KeyF9:        0xFFC6,
		window.KeyF10:       0xFFC7,
		window.KeyF11:       0xFFC8,
		window.KeyF12:       0xFFC9,
	}

	// Handle special keys
	for key, keysym := range keyMap {
		state := f.GetKeyState(key)
		if state == window.KeyStatePressed {
			if err := c.rfbConn.SendKeyEvent(true, keysym); err != nil {
				log.Printf("Failed to send key event: %v", err)
			}
		} else if state == window.KeyStateReleased {
			if err := c.rfbConn.SendKeyEvent(false, keysym); err != nil {
				log.Printf("Failed to send key event: %v", err)
			}
		}
	}

	// Handle letter keys
	for key := window.KeyA; key <= window.KeyZ; key++ {
		state := f.GetKeyState(key)
		if state == window.KeyStatePressed {
			keysym := uint32('a' + (key - window.KeyA))
			if err := c.rfbConn.SendKeyEvent(true, keysym); err != nil {
				log.Printf("Failed to send key event: %v", err)
			}
		} else if state == window.KeyStateReleased {
			keysym := uint32('a' + (key - window.KeyA))
			if err := c.rfbConn.SendKeyEvent(false, keysym); err != nil {
				log.Printf("Failed to send key event: %v", err)
			}
		}
	}

	// Handle number keys
	for key := window.Key0; key <= window.Key9; key++ {
		state := f.GetKeyState(key)
		if state == window.KeyStatePressed {
			keysym := uint32('0' + (key - window.Key0))
			if err := c.rfbConn.SendKeyEvent(true, keysym); err != nil {
				log.Printf("Failed to send key event: %v", err)
			}
		} else if state == window.KeyStateReleased {
			keysym := uint32('0' + (key - window.Key0))
			if err := c.rfbConn.SendKeyEvent(false, keysym); err != nil {
				log.Printf("Failed to send key event: %v", err)
			}
		}
	}
}

var solidColorTextures = make(map[color.Color]graphics.Texture)
var textureMutex sync.Mutex

func (c *vncClient) createSolidColorTexture(col color.Color) graphics.Texture {
	textureMutex.Lock()
	defer textureMutex.Unlock()

	if tex, ok := solidColorTextures[col]; ok {
		return tex
	}

	img := image.NewNRGBA(image.Rect(0, 0, 1, 1))
	img.Set(0, 0, col)
	tex, err := c.gfx.NewTexture(img)
	if err != nil {
		return nil
	}

	solidColorTextures[col] = tex
	return tex
}
