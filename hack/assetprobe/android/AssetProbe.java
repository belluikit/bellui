package bellui.probe;

import android.content.Context;
import android.content.res.AssetFileDescriptor;
import android.content.res.AssetManager;

import java.io.IOException;
import java.io.InputStream;
import java.util.zip.CRC32;

// AssetProbe reads an asset whole through AssetManager.open, which works for
// stored and deflated entries alike, and tries openFd, which works only for a
// stored one.
public final class AssetProbe {
	private AssetProbe() {}

	public static String read(Context ctx, String name) {
		AssetManager am = ctx.getAssets();
		StringBuilder out = new StringBuilder();
		try (InputStream in = am.open(name)) {
			CRC32 crc = new CRC32();
			byte[] buf = new byte[65536];
			long n = 0;
			for (int r; (r = in.read(buf)) > 0; ) {
				crc.update(buf, 0, r);
				n += r;
			}
			out.append(String.format("java=%d/%08x", n, crc.getValue()));
		} catch (IOException e) {
			out.append("java=error:").append(e.getMessage());
		}
		try (AssetFileDescriptor fd = am.openFd(name)) {
			out.append(" fd=").append(fd.getLength());
		} catch (IOException e) {
			// "This file can not be opened as a file descriptor; it is probably compressed"
			out.append(e.getMessage() != null && e.getMessage().contains("compressed") ? " fd=compressed" : " fd=error:" + e.getMessage());
		}
		return out.toString();
	}
}
