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
`WithVolume`, `WithOnEnd`, `WithOnError`, `WithOnTimeUpdate`. (`WithDownmix` is
deprecated and does nothing: surround is downmixed to stereo by default.)

A stream with audio fails its constructor when the audio device will not open
(no device, as on a headless box, or one that refuses the format) or its
decoder cannot convert it to the device's format. The device is opened once per
process, so after one failure every later player with audio returns the same
error. A video with no audio stream never touches the device.

## Audio output

The audio device plays 48 kHz stereo whatever the stream, and each stream's
decoder (FFmpeg's swresample) converts to it: 5.1 and 7.1 are downmixed to
stereo, and mono plays on both channels. Stereo is the default because any
output takes it, and because the device is opened once per process: a device
that refused a surround format used to fail every later player too, stereo
files included ([#707](https://github.com/belluikit/bellui/issues/707)).

An app that wants surround output opts in, before its first player with audio:

```go
if err := mmedia.SetAudioDeviceChannels(6); err != nil { // 5.1
	return err
}
```

After the first player with audio has opened the device, the call returns an
error unless the count is the one already in use; the device is never
reopened. If the output refuses the count, every player with audio fails with
the device's error, so ask for surround only where the output is known to take
it.

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

`LoadWebVTT` is the other parser. To show cues from anywhere else, build a
`SubtitleTrack` of `SubtitleCue`s and pass it to `LoadTrack`. `SetPosition`
drives the overlay by hand if you are not using the helper.

## Painters

The video surface draws through a streaming `canvas.Raster`, and every painter
updates it in place: **WebGPU** (the default on desktop) writes each frame into
the existing texture ([#34](https://github.com/belluikit/bellui/issues/34),
[#100](https://github.com/belluikit/bellui/issues/100)), **GL** (`-tags gl`)
uses `TexSubImage2D`, and the software painter redraws the current pixels on
each paint. The raster still carries a generator, because the WebGPU painter
calls it unconditionally and a nil one crashes on the first paint.

The two GPU painters are separate code paths, so a change to `videosurface.go`
is still worth testing both ways: `go test` and `go test -tags gl`, and on a
GPU, `hack/playprobe -auto` under each.

## Platform support

Playback needs a `goffmpeg-libs` module with linkable FFmpeg artifacts for the
target. Eleven platforms are published (darwin, linux, windows, android, iOS
and the iOS Simulator, amd64/arm64/arm as applicable); the module graph resolves
only the matching one.

## Provenance

Migrated from `trendvidia/fyplayer`. See `PROVENANCE.md` for the source ref, the
exact rewrite set, and the two things this move changed beyond a rename.
