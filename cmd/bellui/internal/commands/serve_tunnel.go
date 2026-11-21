package commands

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

// tunnelURL matches the address cloudflared prints for a quick tunnel. It is
// taken only after the line announcing the tunnel, since cloudflared's own
// service addresses (api. and login.trycloudflare.com) match it too.
var tunnelURL = regexp.MustCompile(`https://[a-z0-9-]+\.trycloudflare\.com`)

// tunnelCreated is in the line cloudflared prints just before the address,
// "Your quick Tunnel has been created! Visit it at ..." (or "protected quick
// Tunnel"), as in cloudflared 2026.10.7.
const tunnelCreated = "Tunnel has been created"

// tunnelStartTimeout bounds how long serve --tunnel waits for cloudflared to
// say where the app is.
var tunnelStartTimeout = 60 * time.Second

// startTunnel runs a Cloudflare quick tunnel to origin and returns its public
// https address, which carries a certificate browsers trust, so the app gets a
// secure page and WebGPU on any device (#1359). It needs no Cloudflare account.
// The tunnel runs until ctx ends; done reports how cloudflared exited.
func startTunnel(ctx context.Context, cloudflared, origin string) (url string, done <-chan error, err error) {
	args := []string{"tunnel", "--no-autoupdate", "--url", origin}
	if strings.HasPrefix(origin, "https://") {
		// The origin is serve's own self-signed certificate.
		args = append(args, "--no-tls-verify")
	}
	cmd := exec.CommandContext(ctx, cloudflared, args...)
	// It logs to stderr; stdout is read too, so a release that moves its log
	// still gives the address.
	output, w, err := os.Pipe()
	if err != nil {
		return "", nil, err
	}
	cmd.Stdout, cmd.Stderr = w, w
	err = cmd.Start()
	_ = w.Close()
	if err != nil {
		_ = output.Close()
		return "", nil, fmt.Errorf("starting cloudflared: %w", err)
	}

	found := make(chan string, 1)
	var log strings.Builder // what cloudflared said before the address, for an error
	exited := make(chan error, 1)
	go func() {
		defer output.Close()
		sc := bufio.NewScanner(output)
		created, seen := false, false
		for sc.Scan() {
			line := sc.Text()
			if seen {
				continue // keep draining, so cloudflared never blocks on a full pipe
			}
			if created {
				if u := tunnelURL.FindString(line); u != "" {
					seen = true
					found <- u
					continue
				}
			}
			created = created || strings.Contains(line, tunnelCreated)
			if log.Len() < 8<<10 {
				log.WriteString(line + "\n")
			}
		}
		_, _ = io.Copy(io.Discard, output)
		exited <- cmd.Wait()
	}()

	select {
	case u := <-found:
		return u, exited, nil
	case err := <-exited:
		return "", nil, fmt.Errorf("cloudflared exited before opening a tunnel (%v):\n%s", err, log.String())
	case <-time.After(tunnelStartTimeout):
		_ = cmd.Process.Kill()
		<-exited
		return "", nil, errors.New("cloudflared did not open a tunnel within " + tunnelStartTimeout.String())
	}
}
