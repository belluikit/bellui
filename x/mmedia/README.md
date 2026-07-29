# Bellui X — mmedia

Media playback widgets for Bellui: video, audio and timed subtitles, decoded by
FFmpeg through [`belluikit/goffmpeg`](https://github.com/belluikit/goffmpeg).

```
go get github.com/belluikit/bellui/x/mmedia
```

## Widgets

| Widget | Constructor | What it is |
|---|---|---|
| `MediaPlayer` | `NewMediaPlayer` / `NewMediaPlayerURL` | video surface + audio output + transport controls, A/V synced |
| `VideoPlayer` | `NewVideoPlayer` / `NewVideoPlayerURL` | video only |
| `AudioPlayer` | `NewAudioPlayer` / `NewAudioPlayerURL` | audio only, with transport controls |
| `SubtitleOverlay` | `NewSubtitleOverlay` | SRT / WebVTT rendered over a player |
| `TransportControls` | `NewTransportControls` | play/pause, seek, mute, volume, time display |

```go
mp, err := mmedia.NewMediaPlayer(f, mmedia.WithAutoPlay())
if err != nil {
	return err
}
defer mp.Close()
w.SetContent(mp)
```

Every constructor takes the same functional options: `WithAutoPlay`, `WithLoop`,
`WithVolume`, `WithDownmix`, `WithOnEnd`, `WithOnError`, `WithOnTimeUpdate`.

## Reader or URL

The two constructor forms are not interchangeable.

`New*Player(io.Reader)` reads through Go. The container and codecs are probed
automatically; a reader that also implements `io.Seeker` supports seeking, and a
plain stream is forward-only.

`New*PlayerURL(string)` hands a file path or network URL to FFmpeg and lets it
own the I/O. **This is the form for anything that fetches its own data** — HLS,
RTSP, RTMP — because a playlist read as a byte stream is only text. It is also
the form that can seek inside a network stream, via range requests. `http`,
`https`, the `hls` and `rtsp` demuxers and the `rtmp` protocol are compiled in.

## Subtitles

`MediaPlayerWithSubtitles` wires an overlay to a player so it follows playback
time, returning the composed container:

```go
so := mmedia.NewSubtitleOverlay()
if err := so.LoadSRT(srt); err != nil {
	return err
}
w.SetContent(mmedia.MediaPlayerWithSubtitles(mp, so))
```

`LoadWebVTT` and `LoadTrack` are the other entry points; `SetPosition` drives
the overlay by hand if you are not using the helper.

## Painters

The video surface takes a different path under each painter, selected at build
time:

* **WebGPU** (the default on desktop) has no streaming path for
  `canvas.Raster`, so frames go through the raster generator. Setting
  `Raster.Streaming` here would not merely fail to help — it would freeze the
  video on its first frame, because `Canvas.freeObject` skips releasing a
  streaming raster's texture on the assumption the painter updates it in place.
* **GL** (`-tags gl`) implements streaming, so `SetPixels` updates the existing
  texture via `TexSubImage2D`.

A change to `videosurface.go` therefore has to be built and tested **both**
ways. See `PROVENANCE.md`.

## Platform support

Playback needs a `goffmpeg-libs` module with linkable FFmpeg artifacts for the
target. Eleven platforms are published (darwin, linux, windows, android, iOS
and the iOS Simulator, amd64/arm64/arm as applicable); the module graph resolves
only the matching one.

## Provenance

Migrated from `trendvidia/fyplayer`. See `PROVENANCE.md` for the source ref, the
exact rewrite set, and the two things this move changed beyond a rename.
