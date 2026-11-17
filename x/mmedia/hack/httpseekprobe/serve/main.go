// Command serve is httpseekprobe's host side. It serves the MP3s in a
// directory twice: on :8090 honouring Range, and on :8091 ignoring it, as
// Python's http.server does. Each request is logged with its leg, its Range
// header, who made it, and how many bytes went out, which is what shows
// whether a seek asked for its bytes or read the file forward.
//
//	go run ./serve <dir-with-cbr.mp3-and-vbr.mp3>
//	adb reverse tcp:8090 tcp:8090 && adb reverse tcp:8091 tcp:8091
package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
)

type counter struct {
	http.ResponseWriter
	n int64
}

func (c *counter) Write(p []byte) (int, error) {
	n, err := c.ResponseWriter.Write(p)
	c.n += int64(n)
	return n, err
}

func handler(dir string, honour bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f, err := os.Open(filepath.Join(dir, filepath.Base(r.URL.Path)))
		if err != nil {
			http.NotFound(w, r)
			return
		}
		defer f.Close()
		st, _ := f.Stat()
		c := &counter{ResponseWriter: w}
		if honour {
			http.ServeContent(c, r, st.Name(), st.ModTime(), f)
		} else {
			w.Header().Set("Content-Type", "audio/mpeg")
			w.Header().Set("Content-Length", strconv.FormatInt(st.Size(), 10))
			_, _ = io.Copy(c, f)
		}
		log.Printf("serve honour=%v leg=%s path=%s range=%q ua=%q sent=%d of %d",
			honour, r.URL.Query().Get("leg"), r.URL.Path, r.Header.Get("Range"), r.UserAgent(), c.n, st.Size())
	})
}

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: serve <dir>")
		os.Exit(2)
	}
	dir := os.Args[1]
	go func() { log.Fatal(http.ListenAndServe(":8091", handler(dir, false))) }()
	log.Fatal(http.ListenAndServe(":8090", handler(dir, true)))
}
