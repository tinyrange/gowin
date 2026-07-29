//go:build darwin

package window

import (
	"errors"
	"runtime"
	"sync"

	"github.com/ebitengine/purego/objc"
	"github.com/tinyrange/gowin/gl"
)

type sharedOpenGLRequest struct {
	run  func(gl.OpenGL) error
	done chan error
}

type darwinSharedOpenGLContext struct {
	owner     *Cocoa
	requests  chan sharedOpenGLRequest
	stop      chan struct{}
	done      chan struct{}
	closeOnce sync.Once
}

func (c *Cocoa) NewSharedOpenGLContext() (SharedOpenGLContext, error) {
	c.sharedMu.Lock()
	if c.closing || c.ctx == 0 || c.pixelFormat == 0 {
		c.sharedMu.Unlock()
		return nil, errors.New("window OpenGL context is closed")
	}
	context := &darwinSharedOpenGLContext{
		owner:    c,
		requests: make(chan sharedOpenGLRequest),
		stop:     make(chan struct{}),
		done:     make(chan struct{}),
	}
	c.sharedContexts[context] = struct{}{}
	c.sharedMu.Unlock()

	ready := make(chan error, 1)
	go context.renderLoop(c.pixelFormat, c.ctx, ready)
	if err := <-ready; err != nil {
		_ = context.Close()
		return nil, err
	}
	return context, nil
}

func (c *Cocoa) removeSharedOpenGLContext(context *darwinSharedOpenGLContext) {
	c.sharedMu.Lock()
	delete(c.sharedContexts, context)
	c.sharedMu.Unlock()
}

func (c *darwinSharedOpenGLContext) Run(run func(gl.OpenGL) error) error {
	if run == nil {
		return errors.New("shared OpenGL context callback is nil")
	}
	request := sharedOpenGLRequest{
		run:  run,
		done: make(chan error, 1),
	}
	select {
	case c.requests <- request:
	case <-c.stop:
		return errors.New("shared OpenGL context is closed")
	case <-c.done:
		return errors.New("shared OpenGL context is closed")
	}
	select {
	case err := <-request.done:
		return err
	case <-c.done:
		return errors.New("shared OpenGL context closed while running callback")
	}
}

func (c *darwinSharedOpenGLContext) Close() error {
	c.closeOnce.Do(func() {
		close(c.stop)
		<-c.done
		c.owner.removeSharedOpenGLContext(c)
	})
	return nil
}

func (c *darwinSharedOpenGLContext) renderLoop(pixelFormat, shareContext objc.ID, ready chan<- error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	defer close(c.done)

	pool := objc.ID(objc.GetClass("NSAutoreleasePool")).Send(selAlloc)
	pool = pool.Send(selInit)
	if pool == 0 {
		ready <- errors.New("failed to create shared OpenGL autorelease pool")
		return
	}
	defer pool.Send(selRelease)

	context := objc.ID(objc.GetClass("NSOpenGLContext")).Send(selAlloc)
	context = context.Send(selInitWithFormat, pixelFormat, shareContext)
	if context == 0 {
		ready <- errors.New("failed to create shared OpenGL context")
		return
	}
	defer context.Send(selRelease)

	context.Send(selMakeCurrentContext)
	api, err := gl.Load()
	if err != nil {
		ready <- err
		objc.ID(objc.GetClass("NSOpenGLContext")).Send(selClearCurrentContext)
		return
	}
	ready <- nil

	defer objc.ID(objc.GetClass("NSOpenGLContext")).Send(selClearCurrentContext)
	for {
		select {
		case request := <-c.requests:
			context.Send(selMakeCurrentContext)
			request.done <- request.run(api)
		case <-c.stop:
			return
		}
	}
}
