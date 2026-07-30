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
// [NewVideoPlayer] and [NewAudioPlayer] expose the single-medium widgets, and
// [NewSubtitleOverlay] renders SRT or WebVTT tracks over a player;
// [MediaPlayerWithSubtitles] wires the two together so the overlay follows
// playback time.
package mmedia
