// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build darwin && (arm || arm64)

package mobileinit

import (
	"bufio"
	"log"
	"os"
	"strings"
	"syscall"
	"unsafe"
)

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Foundation

#include <stdlib.h>

void log_wrap(const char *logStr);
void log_line(const char *line, int isErr);
*/
import "C"

// Store the writer ends of the redirected stdout and stderr so that they are
// not garbage collected and closed.
var stdout, stderr *os.File

// logWriter sends package log output to NSLog, which a console (Xcode,
// devicectl --console) shows. When stderr was redirected it writes to the
// unified log directly instead: NSLog writes to stderr rather than the log
// whenever stderr is not /dev/null, so it would route every line back through
// the pipe at the error level.
type logWriter struct{}

func (logWriter) Write(p []byte) (n int, err error) {
	if stderr != nil {
		cstr := C.CString(strings.TrimSuffix(string(p), "\n"))
		C.log_line(cstr, 0)
		C.free(unsafe.Pointer(cstr))
		return len(p), nil
	}
	cstr := C.CString(string(p))
	C.log_wrap(cstr)
	C.free(unsafe.Pointer(cstr))
	return len(p), nil
}

func init() {
	log.SetOutput(logWriter{})

	// An app started from the home screen has stdout and stderr on
	// /dev/null, so what Go writes there is lost; send it to the unified log
	// instead, line by line, as Android sends it to logcat (#914). Under
	// Xcode or `devicectl --console` they are a terminal that already shows
	// it, and are left alone.
	stdout = redirectToLog(os.Stdout, false)
	stderr = redirectToLog(os.Stderr, true)
}

// redirectToLog points f's descriptor at a pipe whose lines go to the unified
// log, when f is /dev/null. It returns the pipe's writer, or nil when f was
// left alone.
func redirectToLog(f *os.File, isErr bool) *os.File {
	if !isDevNull(int(f.Fd())) {
		return nil
	}
	r, w, err := os.Pipe()
	if err != nil {
		return nil
	}
	if err := syscall.Dup2(int(w.Fd()), int(f.Fd())); err != nil {
		r.Close()
		w.Close()
		return nil
	}
	go lineLog(r, isErr)
	return w
}

// isDevNull reports whether fd is open on /dev/null.
func isDevNull(fd int) bool {
	var st, null syscall.Stat_t
	if syscall.Fstat(fd, &st) != nil || syscall.Stat(os.DevNull, &null) != nil {
		return false
	}
	return st.Mode&syscall.S_IFMT == syscall.S_IFCHR && st.Rdev == null.Rdev
}

// lineLog copies r to the unified log a line at a time. A line longer than
// the buffer goes in pieces, so the pipe is always drained and a writer never
// blocks on it.
func lineLog(r *os.File, isErr bool) {
	flag := C.int(0)
	if isErr {
		flag = 1
	}
	br := bufio.NewReaderSize(r, 1024) // os_log truncates longer messages
	for {
		line, _, err := br.ReadLine()
		if err != nil {
			return
		}
		cstr := C.CString(string(line))
		C.log_line(cstr, flag)
		C.free(unsafe.Pointer(cstr))
	}
}
