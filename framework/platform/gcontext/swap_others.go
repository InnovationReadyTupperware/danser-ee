//go:build !windows

package gcontext

import (
	"log"
	"sync"

	"github.com/Zyko0/go-sdl3/sdl"
)

var swapWarnOnce sync.Once

func SetSwapInterval(interval int) {
	if err := sdl.GL_SetSwapInterval(int32(interval)); err != nil {
		swapWarnOnce.Do(func() {
			log.Printf("OpenGL: failed to set swap interval %d: %v", interval, err)
		})
	}
}

func SwapBuffers() {
	if err := sdl.GL_SwapWindow(sdlWindow); err != nil {
		panic(err)
	}
}
