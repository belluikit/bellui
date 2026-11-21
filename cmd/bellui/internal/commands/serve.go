package commands

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/urfave/cli/v2"
)

// Server serve bellui wasm application over http
type Server struct {
	*appData
	debug       bool
	https       bool
	tunnel      bool
	port        int
	srcDir, dir string
}

// Serve return the cli command for serving bellui wasm application over http
func Serve() *cli.Command {
	s := &Server{appData: &appData{}}

	return &cli.Command{
		Name:    "serve",
		Aliases: []string{"s"},
		Usage:   "Packages an application using WebAssembly and exposes it via a web server",
		Description: `The serve command packages an application using WebAssembly and expose it via a web server which port can be overridden with port.

Browsers offer WebGPU only to a secure page: https, or localhost. To open the app from another device, such as a phone, use --https (a self-signed certificate, whose warning the browser asks you to accept) or --tunnel (a public https address through a Cloudflare quick tunnel; needs cloudflared).`,
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:        "debug",
				Usage:       "enable compiling in debug mode",
				Destination: &s.debug,
			},
			&cli.BoolFlag{
				Name:        "https",
				Usage:       "serve over https with a self-signed certificate, so another device on the network gets a secure page",
				Destination: &s.https,
			},
			&cli.BoolFlag{
				Name:        "tunnel",
				Usage:       "also open a Cloudflare quick tunnel and print its public https address (needs cloudflared; anyone with the address can open the app)",
				Destination: &s.tunnel,
			},
			stringFlags["src"](&s.srcDir),
			stringFlags["icon"](&s.icon),
			intFlags["http-port"](&s.port),
		},
		Action: s.Server,
	}
}

func (s *Server) requestPackage() error {
	p := &Packager{
		os:      "wasm",
		srcDir:  s.srcDir,
		appData: s.appData,
		release: !s.debug,
	}

	if s.debug {
		p.tags = "debug"
	}

	err := p.Package()
	s.dir = p.dir
	return err
}

func (s *Server) serve(ctx context.Context) error {
	err := s.validate()
	if err != nil {
		return err
	}

	err = s.requestPackage()
	if err != nil {
		return err
	}

	webDir := util.EnsureSubDir(s.dir, "wasm")
	ln, err := net.Listen("tcp", ":"+strconv.Itoa(s.port))
	if err != nil {
		return err
	}
	// Stop on Ctrl-C or kill, so a tunnel goes down with serve: a plain kill
	// left cloudflared running, and the app public, after serve ended
	// (#1359). Only from here: packaging does not watch ctx, and a signal
	// must still end it.
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()
	return s.run(ctx, ln, http.FileServer(http.Dir(webDir)), os.Stdout)
}

// run serves handler on ln until the server fails or ctx ends, over https
// when asked, and through a tunnel when asked, printing where the app is.
func (s *Server) run(ctx context.Context, ln net.Listener, handler http.Handler, out io.Writer) error {
	var cloudflared string
	if s.tunnel {
		var err error
		if cloudflared, err = exec.LookPath("cloudflared"); err != nil {
			return errors.New("--tunnel needs cloudflared, Cloudflare's tunnel client, on the PATH")
		}
	}
	port := ln.Addr().(*net.TCPAddr).Port
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	srv := &http.Server{Handler: handler, ReadHeaderTimeout: 10 * time.Second}
	scheme := "http"
	var cert tls.Certificate
	if s.https {
		var err error
		if cert, err = selfSignedCert(networkIPs(), time.Now()); err != nil {
			return fmt.Errorf("making a certificate for --https: %w", err)
		}
		srv.TLSConfig = &tls.Config{Certificates: []tls.Certificate{cert}}
		ln = tls.NewListener(ln, srv.TLSConfig)
		scheme = "https"
	}
	fmt.Fprint(out, servingMessage(s.appData, scheme, port))
	if s.https {
		for _, ip := range networkIPs() {
			if ip.To4() != nil {
				fmt.Fprintf(out, "  on the network: https://%s\n", net.JoinHostPort(ip.String(), strconv.Itoa(port)))
			}
		}
		fmt.Fprintf(out, "The certificate is self-signed, so browsers warn before opening the page.\nIts SHA-256 fingerprint: %s\n", certFingerprint(cert))
	}

	served := make(chan error, 1)
	go func() { served <- srv.Serve(ln) }()
	stopped := ctx.Done()
	go func() {
		<-stopped
		_ = srv.Close()
	}()

	var tunnelDone <-chan error
	if s.tunnel {
		url, done, err := startTunnel(ctx, cloudflared, fmt.Sprintf("%s://127.0.0.1:%d", scheme, port))
		if err != nil {
			return err
		}
		tunnelDone = done
		fmt.Fprintf(out, "Public tunnel: %s\nAnyone with this address can open the app while serve runs.\n", url)
	}
	select {
	case err := <-served:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case err := <-tunnelDone:
		if ctx.Err() != nil {
			return nil
		}
		return fmt.Errorf("the tunnel closed: cloudflared exited (%v)", err)
	}
}

// servingMessage is the line serve prints once the app is up. It names the app
// by its ID, or by its name when it has none. A web build has no ID when there
// is no manifest or the manifest sets none (serve has no --app-id), and the
// line then read "Serving  at:" (#703).
func servingMessage(app *appData, scheme string, port int) string {
	name := app.AppID
	if name == "" {
		name = app.Name
	}
	return fmt.Sprintf("Serving %s at: %s://localhost:%v\n", name, scheme, port)
}

// Server serve the wasm application over http
func (s *Server) Server(ctx *cli.Context) error {
	if ctx.Args().Len() != 0 {
		return errors.New("unexpected parameter after flags")
	}

	return s.serve(ctx.Context)
}

func (s *Server) validate() error {
	if s.port == 0 {
		s.port = 8080
	}
	if s.port < 0 || s.port > 65535 {
		return fmt.Errorf("the port must be a strictly positive number and be strictly smaller than 65536 (Got %v)", s.port)
	}
	return nil
}
