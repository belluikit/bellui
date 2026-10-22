package bellui.probe;

import android.content.Context;

import androidx.collection.ArraySet;
import androidx.core.content.ContextCompat;

// LibProbe exercises the android/libs dependencies from app Java: an AndroidX
// API (ContextCompat), resources that live in an AAR read through that
// library's own generated R class, and a class from a plain jar (ArraySet).
public final class LibProbe {
	private LibProbe() {}

	public static String probe(Context ctx) {
		int color = ContextCompat.getColor(ctx, androidx.core.R.color.notification_icon_bg_color);
		String overflow = ctx.getString(androidx.core.R.string.status_bar_notification_info_overflow);
		ArraySet<String> set = new ArraySet<>();
		set.add("a");
		set.add("b");
		set.add("a");
		return String.format("color=#%08x overflow=%s arrayset=%d", color, overflow, set.size());
	}
}
