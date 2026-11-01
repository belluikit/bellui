// Command serve is httpsprobe's local leg: it serves one file over https with
// a self-signed certificate, honouring Range, and logs each request's range
// and the bytes it sent, which is the server's side of the probe's count.
//
// Make a file with its index at the end, the shape that needs ranges (FFmpeg
// writes moov last unless told -movflags +faststart):
//
//	ffmpeg -f lavfi -i testsrc=duration=600:size=640x360:rate=30 \
//	       -f lavfi -i sine=frequency=440:duration=600 -shortest \
//	       -c:v libx264 -preset veryfast -c:a aac big.mp4
//	go run ./serve big.mp4
//	adb reverse tcp:8443 tcp:8443
package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"log"
	"math/big"
	"net/http"
	"os"
	"sync/atomic"
	"time"
)

func main() {
	if len(os.Args) != 2 {
		log.Fatal("usage: serve <file>")
	}
	path := os.Args[1]
	var total atomic.Int64
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f, err := os.Open(path)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer f.Close()
		cw := &countingWriter{ResponseWriter: w}
		start := time.Now()
		http.ServeContent(cw, r, "big.mp4", time.Time{}, f)
		log.Printf("range %q sent %d bytes in %v (all requests: %d)",
			r.Header.Get("Range"), cw.n, time.Since(start).Round(time.Millisecond), total.Add(cw.n))
	})
	srv := &http.Server{Addr: ":8443", Handler: h, TLSConfig: &tls.Config{Certificates: []tls.Certificate{selfSigned()}}}
	log.Printf("serving %s on https://localhost:8443/big.mp4", path)
	log.Fatal(srv.ListenAndServeTLS("", ""))
}

type countingWriter struct {
	http.ResponseWriter
	n int64
}

func (w *countingWriter) Write(p []byte) (int, error) {
	n, err := w.ResponseWriter.Write(p)
	w.n += int64(n)
	return n, err
}

func selfSigned() tls.Certificate {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		log.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "localhost"},
		DNSNames:     []string{"localhost"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		log.Fatal(err)
	}
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}
}
