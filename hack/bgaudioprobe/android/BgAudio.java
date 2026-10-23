package bellui.probe;

import android.app.ActivityManager;
import android.content.Context;
import android.media.AudioAttributes;
import android.media.AudioFormat;
import android.media.AudioTrack;

import java.util.concurrent.atomic.AtomicLong;

// BgAudio plays the probe's beep through an AudioTrack on its own thread and
// counts the frames it has written, so the probe can tell whether audio
// keeps flowing after the app leaves the screen.
public final class BgAudio {
	private static final int RATE = 48000;
	private static final AtomicLong frames = new AtomicLong();
	private static volatile AudioTrack track;

	private BgAudio() {}

	// start returns "" on success, or an error message.
	public static String start(Context ctx) {
		try {
			int min = AudioTrack.getMinBufferSize(RATE, AudioFormat.CHANNEL_OUT_MONO, AudioFormat.ENCODING_PCM_16BIT);
			AudioTrack t = new AudioTrack.Builder()
				.setAudioAttributes(new AudioAttributes.Builder()
					.setUsage(AudioAttributes.USAGE_MEDIA)
					.setContentType(AudioAttributes.CONTENT_TYPE_MUSIC)
					.build())
				.setAudioFormat(new AudioFormat.Builder()
					.setSampleRate(RATE)
					.setEncoding(AudioFormat.ENCODING_PCM_16BIT)
					.setChannelMask(AudioFormat.CHANNEL_OUT_MONO)
					.build())
				.setBufferSizeInBytes(Math.max(min, RATE / 5 * 2))
				.setTransferMode(AudioTrack.MODE_STREAM)
				.build();
			t.play();
			track = t;
			Thread th = new Thread(BgAudio::loop, "bgaudioprobe");
			th.setDaemon(true);
			th.start();
			return "";
		} catch (Exception e) {
			return e.toString();
		}
	}

	private static void loop() {
		short[] buf = new short[RATE / 50]; // 20 ms
		long pos = 0;
		double phase = 0;
		while (true) {
			for (int i = 0; i < buf.length; i++, pos++) {
				boolean on = (pos / (RATE / 2)) % 2 == 0; // 0.5 s on, 0.5 s off
				buf[i] = on ? (short) (0.08 * Short.MAX_VALUE * Math.sin(phase)) : 0;
				phase += 2 * Math.PI * 440 / RATE;
				if (phase > 2 * Math.PI) phase -= 2 * Math.PI;
			}
			int n = track.write(buf, 0, buf.length); // blocks while the buffer is full
			if (n < 0) return;
			frames.addAndGet(n);
		}
	}

	// stats reports "<importance> <frames> <playing>" for the probe's log line.
	public static String stats(Context ctx) {
		ActivityManager.RunningAppProcessInfo info = new ActivityManager.RunningAppProcessInfo();
		ActivityManager.getMyMemoryState(info);
		String imp;
		switch (info.importance) {
			case ActivityManager.RunningAppProcessInfo.IMPORTANCE_FOREGROUND: imp = "foreground"; break;
			case ActivityManager.RunningAppProcessInfo.IMPORTANCE_FOREGROUND_SERVICE: imp = "fgservice"; break;
			case ActivityManager.RunningAppProcessInfo.IMPORTANCE_VISIBLE: imp = "visible"; break;
			case ActivityManager.RunningAppProcessInfo.IMPORTANCE_PERCEPTIBLE: imp = "perceptible"; break;
			case ActivityManager.RunningAppProcessInfo.IMPORTANCE_SERVICE: imp = "service"; break;
			case ActivityManager.RunningAppProcessInfo.IMPORTANCE_CACHED: imp = "cached"; break;
			default: imp = "importance" + info.importance;
		}
		AudioTrack t = track;
		boolean playing = t != null && t.getPlayState() == AudioTrack.PLAYSTATE_PLAYING;
		return imp + " " + frames.get() + " " + playing;
	}
}
