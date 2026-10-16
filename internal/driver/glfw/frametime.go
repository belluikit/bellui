package glfw

import (
	"fmt"
	"os"
	"sort"
	"sync"
	"time"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/internal/build"
)

// frameProfiler accumulates per-frame paint timings so the partialrepaint
// optimization can be benchmarked on a real GPU, where the GL/WebGPU paint path
// runs but the software-painter golden suite cannot reach it (see pre-rebrand issue #9). It
// is gated behind the BELLUI_FRAME_DEBUG environment variable and is otherwise
// inert — the hot path only pays an atomic-free bool check per frame.
//
// BELLUI_FRAME_DEBUG accepts either "1" (report every 120 frames) or an integer
// frame count to report on (e.g. BELLUI_FRAME_DEBUG=300). Each report prints, for
// both the CPU-side paint() call and the whole paint+present, the sample count
// and the mean/p50/p95/p99/max in microseconds, plus whether the build was
// compiled with the partialrepaint tag. paint() time is the vsync-independent
// signal: it captures the extra full-tree PartialDirtyRegion walk against the
// draw-call/fill issuance saved by culling unchanged objects.
type frameProfiler struct {
	mu      sync.Mutex
	paint   []time.Duration
	total   []time.Duration
	gpu     []time.Duration // on-GPU frame time (BELLUI_GPU_PROFILE), pre-rebrand tracker #336
	report  int
	enabled bool
}

var profiler = newFrameProfiler()

func newFrameProfiler() *frameProfiler {
	v := os.Getenv("BELLUI_FRAME_DEBUG")
	if v == "" {
		return &frameProfiler{}
	}
	report := 120
	if n, err := fmt.Sscanf(v, "%d", &report); n != 1 || err != nil || report <= 0 {
		report = 120
	}
	bellui.LogError(fmt.Sprintf("BELLUI_FRAME_DEBUG: profiling paint frame time, partialrepaint=%v, reporting every %d frames", build.PartialRepaint, report), nil)
	return &frameProfiler{report: report, enabled: true}
}

// record adds one frame's CPU paint duration, full paint+present duration, and
// (when available) the measured on-GPU frame time, emitting a percentile report
// once report-many frames have accumulated. A zero gpu duration is treated as "no
// measurement" and omitted from the GPU column.
func (p *frameProfiler) record(paint, total, gpu time.Duration) {
	if !p.enabled {
		return
	}
	p.mu.Lock()
	p.paint = append(p.paint, paint)
	p.total = append(p.total, total)
	if gpu > 0 {
		p.gpu = append(p.gpu, gpu)
	}
	if len(p.paint) >= p.report {
		p.flushLocked()
	}
	p.mu.Unlock()
}

// Flush prints whatever has accumulated so far (used on shutdown so a run that
// ends before a full reporting window still reports). Safe to call when disabled.
func (p *frameProfiler) Flush() {
	if p == nil || !p.enabled {
		return
	}
	p.mu.Lock()
	p.flushLocked()
	p.mu.Unlock()
}

func (p *frameProfiler) flushLocked() {
	if len(p.paint) == 0 {
		return
	}
	bellui.LogError(fmt.Sprintf("BELLUI_FRAME_DEBUG partialrepaint=%v frames=%d", build.PartialRepaint, len(p.paint)), nil)
	bellui.LogError("  paint()      "+summarize(p.paint), nil)
	bellui.LogError("  paint+present "+summarize(p.total), nil)
	if len(p.gpu) > 0 {
		bellui.LogError(fmt.Sprintf("  gpu (n=%d)    %s", len(p.gpu), summarize(p.gpu)), nil)
	}
	p.paint = p.paint[:0]
	p.total = p.total[:0]
	p.gpu = p.gpu[:0]
}

// summarize returns "mean=… p50=… p95=… p99=… max=…" in microseconds. It sorts a
// copy so the caller's slice order is preserved for any later appends.
func summarize(d []time.Duration) string {
	cp := make([]time.Duration, len(d))
	copy(cp, d)
	sort.Slice(cp, func(i, j int) bool { return cp[i] < cp[j] })

	var sum time.Duration
	for _, v := range cp {
		sum += v
	}
	us := func(v time.Duration) float64 { return float64(v.Nanoseconds()) / 1000 }
	pct := func(f float64) time.Duration {
		idx := int(f * float64(len(cp)-1))
		return cp[idx]
	}
	return fmt.Sprintf("mean=%.1f p50=%.1f p95=%.1f p99=%.1f max=%.1f µs",
		us(sum)/float64(len(cp)), us(pct(0.50)), us(pct(0.95)), us(pct(0.99)), us(cp[len(cp)-1]))
}
