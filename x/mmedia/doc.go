// Package mmedia provides media playback widgets for the Bellui GUI toolkit —
// video, audio and timed subtitles, decoded by FFmpeg through
// [github.com/belluikit/goffmpeg].
//
// [NewMediaPlayer] is the primary entry point: a widget that combines a video
// surface, audio output and transport controls, with A/V sync handled by the
// pipeline behind it.
//
//	mp, err := mmedia.NewMediaPlayer(f, mmedia.WithAutoPlay())
//	if err != nil {
//		return err
//	}
//	defer mp.Close()
//	w.SetContent(mp)
//
// [NewMediaPlayerURL] plays from a file path or network URL instead, letting
// FFmpeg own the I/O. That is the form to use for anything that fetches its own
// data — HLS, RTSP, RTMP — or that needs range-request seeking within a network
// stream.
//
// [WithScrubPreview] shows the frame under the pointer while the seek bar is
// hovered, so a viewer can see where a seek would land before committing to it.
// It needs a source FFmpeg can open twice, which the URL and path forms are.
//
// [NewCapturePlayer] opens a camera instead of a recording, with the same widget
// and controls around it. [CaptureSelect] is the picker for choosing between
// cameras, and [MediaPlayer.SetCaptureDevice] switches one for another inside
// the running player rather than rebuilding it.
//
// A player is not built on audio it cannot play. Every constructor returns an
// error when the stream has audio and the audio device will not open (no device,
// as on a headless machine, or one that refuses the format), or when the
// stream's decoder cannot convert it to the device's format. The device is
// opened once per process, so once it has failed to open, every later player
// with audio gets the same error.
//
// The audio device plays 48 kHz stereo, whatever the stream: each stream's
// decoder converts to it, so 5.1 and 7.1 are downmixed to stereo and mono plays
// on both channels. An app that wants surround output calls
// [SetAudioDeviceChannels] before its first player with audio; after that the
// device's format is fixed and the call returns an error.
//
// [NewVideoPlayer] and [NewAudioPlayer] expose the single-medium widgets, and
// [NewSubtitleOverlay] renders SRT or WebVTT tracks over a player;
// [MediaPlayerWithSubtitles] wires the two together so the overlay follows
// playback time.
package mmedia
