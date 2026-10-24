package mmedia

import "github.com/belluikit/bellui/x/mmedia/internal/pipeline"

// SetAudioDeviceChannels opts in to surround output: it sets the channel count
// the audio device is opened with, 6 for 5.1 or 8 for 7.1, in place of the
// default, stereo. Without it a surround stream is downmixed to stereo, which
// any output device plays. Every stream is converted to the device's channels
// by FFmpeg's swresample, so with a surround device a stereo stream plays on
// the front left and right.
//
// Call it before the first player with audio is built. A process has one audio
// device, opened by that player and never reopened (the oto library allows
// one), so after that SetAudioDeviceChannels returns an error unless n is the
// count already in use, and the device keeps its count. n must be positive.
//
// A device that refuses n channels fails the first player with audio, and the
// same error is returned by every later one, stereo files included, until the
// process restarts. Ask for surround only for an output known to take it.
func SetAudioDeviceChannels(n int) error {
	return pipeline.SetDeviceChannels(n)
}
