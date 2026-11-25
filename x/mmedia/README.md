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
| `AudioPlayback` | `NewAudioPlayback` / `NewAudioPlaybackURL` | audio with no UI; show it with `NewAudioPlayerFor`, so playback outlives a screen |
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

For a song that keeps playing while screens come and go, open an
`AudioPlayback`, which needs no widget, window or app, and give each screen a
view of it. The view's `Close` (or `Detach`) leaves the song playing; the app
closes the playback when it is done with it.

```go
pb, err := mmedia.NewAudioPlaybackURL(url)
if err != nil {
	return err
}
pb.Play()
w.SetContent(mmedia.NewAudioPlayerFor(pb)) // on each screen that shows it
```

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

`https` on Android is the exception. FFmpeg's Android libraries have no TLS, so
there x/mmedia reads an `https` URL through Go's `net/http` instead, with range
requests, and hands the bytes to FFmpeg. A file still streams and seeks, a
dropped or stalled connection is resumed as FFmpeg would, and the system's
certificates are used, so an app ships no CA bundle. An `https` HLS playlist
cannot be read that way, since its segments are fetched by the demuxer, so it
fails to open on Android with an error that says so. Plain `http` is FFmpeg's
on every platform.

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

### Formats

goffmpeg builds FFmpeg in two codec sets, chosen by a build tag
([goffmpeg#41](https://github.com/belluikit/goffmpeg/issues/41)):

| Set | Plays | Build |
|---|---|---|
| media, desktop | every format and codec FFmpeg decodes; webcams | default |
| media, phones | MP3, WAV, MP4 (`.mp4`, `.m4a`, `.mov`), MPEG-TS, HLS, WebM/Matroska and Ogg, with H.264, VP9, AV1, AAC or Opus | default |
| audio | MP3, WAV, AAC (`.m4a`, `.mp4`, ADTS) and Opus (`.ogg`); no video and no capture | `-tags goffmpeg_novideo` |

On Android and iOS, H.264 and AAC decode only through the platform's own
decoders (MediaCodec; VideoToolbox and AudioToolbox), so a stream they refuse,
such as 10-bit or High 4:4:4 H.264, does not play on a phone (goffmpeg
v0.13.0). Outside its set a file fails to open with
`goffmpeg.ErrUnrecognizedInput`, or `goffmpeg.ErrNoDecoder` when the container
opens but none of its streams decodes. An audio-only app ships less with the
audio set: 2.4 MB of FFmpeg against 15.9 MB on darwin/arm64 (goffmpeg-libs
v0.10.0). Each module's `NOTICE` records the configure line of both sets.

### Shipping FFmpeg

FFmpeg is a set of shared libraries, so a user can replace them (LGPL 2.1 §6b).
An app has to carry the ones its binary loads. `go run` and `go test` find them
in the module cache. `bellui package` copies them into the package and points
the binary at them: `Contents/Frameworks` in a macOS app, `lib/<app id>/` in a
Linux package, beside the `.exe` on Windows
([#1101](https://github.com/belluikit/bellui/issues/1101)), each ABI's
`lib/<abi>/` in an Android APK, and `Frameworks/` in an iOS app, signed with it
([#1122](https://github.com/belluikit/bellui/issues/1122)). Built with
`goffmpeg_novideo`, a package carries only the audio set.

### Background playback

On **iOS**, x/mmedia sets the app's audio session to the Playback category when
it opens the audio device ([#845](https://github.com/belluikit/bellui/issues/845)).
Audio then keeps playing when the app is in the background or the screen is
locked, and it plays through the silent switch, as a music or podcast app does.
iOS only allows the background part when the app declares it, in
`ios/Info.plist` next to `manifest.pxf`:

```xml
<key>UIBackgroundModes</key>
<array>
	<string>audio</string>
</array>
```

Without that key, iOS still stops the audio when the app leaves the
foreground (Apple's rule for the category, not measured here).

An app that is recording from the microphone when x/mmedia opens the device
keeps its recording: a session in the Record category moves to PlayAndRecord
instead, playing from the speaker rather than the earpiece, and one already in PlayAndRecord is
left as the app set it ([#1188](https://github.com/belluikit/bellui/issues/1188)).
Before this, iOS refused the change to Playback while the capture ran, and the
first song failed to open.

On **Android**, playback continued in the background and on the lock screen
with no extra setup on the one tablet measured (2026-10-27). Android may still
freeze an app that sits in the background without playing.

## Provenance

Migrated from `trendvidia/fyplayer`. See `PROVENANCE.md` for the source ref, the
exact rewrite set, and the two things this move changed beyond a rename.
