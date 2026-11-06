//go:build darwin

package main

import (
	"fmt"
	"os"
	"runtime"
	"slices"
	"time"

	"github.com/belluikit/glfw"
)

func init() { runtime.LockOSThread() }

func main() {
	if err := glfw.Init(); err != nil {
		panic(err)
	}
	if os.Getenv("RAW_WINDOW") == "1" {
		if _, err := glfw.CreateWindow(200, 200, "raw", nil, nil); err != nil {
			panic(err)
		}
	}
	glfw.PollEvents()
	posts := make(chan time.Time, 1000)
	go func() {
		for range 200 {
			time.Sleep(5 * time.Millisecond)
			posts <- time.Now()
			glfw.PostEmptyEvent()
		}
	}()
	var lat []time.Duration
	deadline := time.Now().Add(10 * time.Second)
	for len(lat) < 200 && time.Now().Before(deadline) {
		glfw.WaitEventsTimeout(0.5)
		now := time.Now()
		for len(posts) > 0 {
			lat = append(lat, now.Sub(<-posts))
		}
	}
	slices.Sort(lat)
	n := len(lat)
	fmt.Println("raw n", n, "post->wake p10", lat[n/10], "p50", lat[n/2], "p90", lat[n*9/10], "max", lat[n-1])
}
