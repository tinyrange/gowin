//go:build darwin

package window

import (
	"bytes"
	"runtime"
	"testing"
	"unsafe"

	"github.com/tinyrange/gowin/gl"
)

func TestSharedOpenGLContextPublishesTexture(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	win, err := New("Gowin shared-context integration", 64, 64, true)
	if err != nil {
		t.Fatal(err)
	}
	defer win.Close()
	api, err := win.GL()
	if err != nil {
		t.Fatal(err)
	}
	synchronization, ok := api.(gl.Synchronization)
	if !ok {
		t.Fatal("presentation context does not support explicit synchronization")
	}
	provider, ok := win.(SharedOpenGLContextProvider)
	if !ok {
		t.Fatal("window does not support shared OpenGL contexts")
	}
	shared, err := provider.NewSharedOpenGLContext()
	if err != nil {
		t.Fatal(err)
	}
	defer shared.Close()

	var texture uint32
	if err := shared.Run(func(producer gl.OpenGL) error {
		producer.GenTextures(1, &texture)
		producer.BindTexture(gl.Texture2D, texture)
		producer.TexParameteri(gl.Texture2D, gl.TextureMinFilter, gl.Nearest)
		producer.TexParameteri(gl.Texture2D, gl.TextureMagFilter, gl.Nearest)
		producer.TexImage2D(gl.Texture2D, 0, int32(gl.RGBA), 2, 2, 0, gl.RGBA, gl.UnsignedByte, nil)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if texture == 0 {
		t.Fatal("producer did not create a texture")
	}

	var framebuffer uint32
	api.GenFramebuffers(1, &framebuffer)
	defer api.DeleteFramebuffers(1, &framebuffer)
	api.BindFramebuffer(gl.Framebuffer, framebuffer)
	api.FramebufferTexture2D(gl.Framebuffer, gl.ColorAttachment0, gl.Texture2D, texture, 0)
	if status := api.CheckFramebufferStatus(gl.Framebuffer); status != gl.FramebufferComplete {
		t.Fatalf("shared texture framebuffer status = %#x", status)
	}

	frameColors := [][]byte{
		{37, 83, 191, 255},
		{241, 197, 38, 255},
	}
	for frame := 0; frame < 16; frame++ {
		want := bytes.Repeat(frameColors[frame%len(frameColors)], 4)
		var fence gl.Sync
		if err := shared.Run(func(producer gl.OpenGL) error {
			producer.BindTexture(gl.Texture2D, texture)
			producer.TexSubImage2D(gl.Texture2D, 0, 0, 0, 2, 2,
				gl.RGBA, gl.UnsignedByte, unsafe.Pointer(&want[0]))
			producerSync := producer.(gl.Synchronization)
			fence = producerSync.FenceSync(gl.SyncGPUCommandsComplete, 0)
			producerSync.Flush()
			return nil
		}); err != nil {
			t.Fatal(err)
		}
		runtime.KeepAlive(want)
		if fence == 0 {
			t.Fatalf("frame %d did not produce a fence", frame)
		}
		synchronization.WaitSync(fence, 0, gl.TimeoutIgnored)
		synchronization.DeleteSync(fence)

		got := make([]byte, len(want))
		api.ReadPixels(0, 0, 2, 2, gl.RGBA, gl.UnsignedByte, unsafe.Pointer(&got[0]))
		runtime.KeepAlive(got)
		if !bytes.Equal(got, want) {
			t.Fatalf("frame %d shared texture pixels = %v, want %v", frame, got, want)
		}
	}
	api.DeleteTextures(1, &texture)
}
