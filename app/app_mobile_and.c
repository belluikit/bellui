//go:build !ci && android

#include <android/log.h>
#include <jni.h>
#include <stdbool.h>
#include <stdlib.h>

#define LOG_FATAL(...) __android_log_print(ANDROID_LOG_FATAL, "Bellui", __VA_ARGS__)

static jclass find_class(JNIEnv *env, const char *class_name) {
	jclass clazz = (*env)->FindClass(env, class_name);
	if (clazz == NULL) {
		(*env)->ExceptionClear(env);
		LOG_FATAL("cannot find %s", class_name);
		return NULL;
	}
	return clazz;
}

static jmethodID find_method(JNIEnv *env, jclass clazz, const char *name, const char *sig) {
	jmethodID m = (*env)->GetMethodID(env, clazz, name, sig);
	if (m == 0) {
		(*env)->ExceptionClear(env);
		LOG_FATAL("cannot find method %s %s", name, sig);
		return 0;
	}
	return m;
}

static jmethodID find_static_method(JNIEnv *env, jclass clazz, const char *name, const char *sig) {
	jmethodID m = (*env)->GetStaticMethodID(env, clazz, name, sig);
	if (m == 0) {
		(*env)->ExceptionClear(env);
		LOG_FATAL("cannot find method %s %s", name, sig);
		return 0;
	}
	return m;
}

jobject getSystemService(uintptr_t jni_env, uintptr_t ctx, char *service) {
	JNIEnv *env = (JNIEnv*)jni_env;
	jstring serviceStr = (*env)->NewStringUTF(env, service);

	jclass ctxClass = (*env)->GetObjectClass(env, (jobject)ctx);
	jmethodID getSystemService = find_method(env, ctxClass, "getSystemService", "(Ljava/lang/String;)Ljava/lang/Object;");

	return (jobject)(*env)->CallObjectMethod(env, (jobject)ctx, getSystemService, serviceStr);
}

int nextId = 1;

bool isOreoOrLater(JNIEnv *env) {
    jclass versionClass = find_class(env, "android/os/Build$VERSION" );
    jfieldID sdkIntFieldID = (*env)->GetStaticFieldID(env, versionClass, "SDK_INT", "I" );
    int sdkVersion = (*env)->GetStaticIntField(env, versionClass, sdkIntFieldID );

    return sdkVersion >= 26; // O = Oreo, will not be defined for older builds
}

int androidSDKInt(JNIEnv *env) {
    jclass versionClass = find_class(env, "android/os/Build$VERSION");
    jfieldID sdkIntFieldID = (*env)->GetStaticFieldID(env, versionClass, "SDK_INT", "I");
    return (*env)->GetStaticIntField(env, versionClass, sdkIntFieldID);
}

// Only prompt once per process: Android won't re-show the dialog after the user
// has answered, but avoid re-issuing requestPermissions while a prompt is up.
static bool notifPermRequested = false;

// ensureNotificationPermission requests POST_NOTIFICATIONS the first time a
// notification is posted on Android 13+ (API 33), where it is a *runtime*
// permission — until it is granted the OS silently drops every notification
// (fyne#756). Below API 33 the permission is granted at install, so this is a
// no-op. bellui does not need the grant result: the request is best-effort and
// the requesting notification may itself be dropped, but the prompt is raised
// so subsequent notifications post once the user grants.
static void ensureNotificationPermission(JNIEnv *env, jobject ctx) {
    if (androidSDKInt(env) < 33) { // TIRAMISU
        return;
    }

    jstring perm = (*env)->NewStringUTF(env, "android.permission.POST_NOTIFICATIONS");

    // checkSelfPermission == PackageManager.PERMISSION_GRANTED (0) -> nothing to do.
    jclass ctxClass = find_class(env, "android/content/Context");
    jmethodID checkSelfPermission = find_method(env, ctxClass, "checkSelfPermission", "(Ljava/lang/String;)I");
    jint granted = (*env)->CallIntMethod(env, ctx, checkSelfPermission, perm);
    if (granted == 0 || notifPermRequested) {
        (*env)->DeleteLocalRef(env, perm);
        return;
    }
    notifPermRequested = true;

    // activity.requestPermissions(new String[]{perm}, 0) — Activity method (API 23+).
    // Resolve requestPermissions from the ctx object's own class (GoNativeActivity
    // extends NativeActivity extends Activity); FindClass can't see app classes on
    // a JVM-attached native thread, but the inherited method is reachable this way.
    jclass strClass = find_class(env, "java/lang/String");
    jobjectArray perms = (*env)->NewObjectArray(env, 1, strClass, perm);

    jclass actClass = (*env)->GetObjectClass(env, ctx);
    jmethodID requestPermissions = find_method(env, actClass, "requestPermissions", "([Ljava/lang/String;I)V");
    (*env)->CallVoidMethod(env, ctx, requestPermissions, perms, (jint)0);
    (*env)->ExceptionClear(env); // best-effort; must never crash the notification path

    (*env)->DeleteLocalRef(env, perms);
    (*env)->DeleteLocalRef(env, perm);
}

jobject parseURL(uintptr_t jni_env, uintptr_t ctx, char* uriCstr) {
	JNIEnv *env = (JNIEnv*)jni_env;

	jstring uriStr = (*env)->NewStringUTF(env, uriCstr);
	jclass uriClass = find_class(env, "android/net/Uri");
	jmethodID parse = find_static_method(env, uriClass, "parse", "(Ljava/lang/String;)Landroid/net/Uri;");

	return (jobject)(*env)->CallStaticObjectMethod(env, uriClass, parse, uriStr);
}

void openURL(uintptr_t java_vm, uintptr_t jni_env, uintptr_t ctx, char *url) {
	JNIEnv *env = (JNIEnv*)jni_env;
	jobject uri = parseURL(jni_env, ctx, url);

	jclass intentClass = find_class(env, "android/content/Intent");
	jfieldID viewFieldID = (*env)->GetStaticFieldID(env, intentClass, "ACTION_VIEW", "Ljava/lang/String;" );
    jstring view = (*env)->GetStaticObjectField(env, intentClass, viewFieldID);

	jmethodID constructor = find_method(env, intentClass, "<init>", "(Ljava/lang/String;Landroid/net/Uri;)V");
	jobject intent = (*env)->NewObject(env, intentClass, constructor, view, uri);

	jclass contextClass = find_class(env, "android/content/Context");
	jmethodID start = find_method(env, contextClass, "startActivity", "(Landroid/content/Intent;)V");
	(*env)->CallVoidMethod(env, (jobject)ctx, start, intent);
}

// shareText hands plain text to the Android share sheet via an ACTION_SEND
// chooser (the "send" half of intent/share integration, fyne#711).
void shareText(uintptr_t java_vm, uintptr_t jni_env, uintptr_t ctx, char *text) {
	JNIEnv *env = (JNIEnv*)jni_env;
	jstring textStr = (*env)->NewStringUTF(env, text);

	jclass intentClass = find_class(env, "android/content/Intent");

	jfieldID sendFieldID = (*env)->GetStaticFieldID(env, intentClass, "ACTION_SEND", "Ljava/lang/String;");
	jstring actionSend = (*env)->GetStaticObjectField(env, intentClass, sendFieldID);
	jfieldID extraTextFieldID = (*env)->GetStaticFieldID(env, intentClass, "EXTRA_TEXT", "Ljava/lang/String;");
	jstring extraText = (*env)->GetStaticObjectField(env, intentClass, extraTextFieldID);

	jmethodID constructor = find_method(env, intentClass, "<init>", "(Ljava/lang/String;)V");
	jobject intent = (*env)->NewObject(env, intentClass, constructor, actionSend);

	jstring mime = (*env)->NewStringUTF(env, "text/plain");
	jmethodID setType = find_method(env, intentClass, "setType", "(Ljava/lang/String;)Landroid/content/Intent;");
	(*env)->CallObjectMethod(env, intent, setType, mime);

	jmethodID putExtra = find_method(env, intentClass, "putExtra", "(Ljava/lang/String;Ljava/lang/CharSequence;)Landroid/content/Intent;");
	(*env)->CallObjectMethod(env, intent, putExtra, extraText, textStr);

	jmethodID createChooser = find_static_method(env, intentClass, "createChooser", "(Landroid/content/Intent;Ljava/lang/CharSequence;)Landroid/content/Intent;");
	jobject chooser = (*env)->CallStaticObjectMethod(env, intentClass, createChooser, intent, NULL);

	jclass contextClass = find_class(env, "android/content/Context");
	jmethodID start = find_method(env, contextClass, "startActivity", "(Landroid/content/Intent;)V");
	(*env)->CallVoidMethod(env, (jobject)ctx, start, chooser);
}

// shareFile hands a staged local file to the Android share sheet. Building the
// content:// URI needs BelluiFileProvider.uriForFile, so the ACTION_SEND is
// assembled in the Java GoNativeActivity.doShareFile rather than here; Go has
// already copied the file into the provider's share directory (fyne#711).
//
// The activity method is resolved from the ctx object's own class via
// GetObjectClass — FindClass by name would fail here, since this runs on a
// JVM-attached native thread whose system class loader cannot see app classes
// (only framework classes, which is why shareText/openURL can FindClass).
void shareFile(uintptr_t java_vm, uintptr_t jni_env, uintptr_t ctx, char *path, char *mime) {
	JNIEnv *env = (JNIEnv*)jni_env;
	jstring pathStr = (*env)->NewStringUTF(env, path);
	jstring mimeStr = (*env)->NewStringUTF(env, mime);

	jclass actClass = (*env)->GetObjectClass(env, (jobject)ctx);
	jmethodID share = find_method(env, actClass, "doShareFile", "(Ljava/lang/String;Ljava/lang/String;)V");
	(*env)->CallVoidMethod(env, (jobject)ctx, share, pathStr, mimeStr);
}

// buildBaseNotification creates a Notification$Builder with the bellui channel (on
// Oreo+), title, body and default small icon set — shared by the plain and rich
// (fyne#723) paths.
static jobject buildBaseNotification(JNIEnv *env, jobject ctx, jclass cls, jstring titleStr, jstring bodyStr) {
	jmethodID constructor = find_method(env, cls, "<init>", "(Landroid/content/Context;)V");
	jobject builder = (*env)->NewObject(env, cls, constructor, ctx);

	if (isOreoOrLater(env)) {
		jclass mgrCls = find_class(env, "android/app/NotificationManager");
		jobject mgr = getSystemService((uintptr_t)env, (uintptr_t)ctx, "notification");

		jstring channelId = (*env)->NewStringUTF(env, "bellui-notif");
		jstring name = (*env)->NewStringUTF(env, "Bellui Notification");
		int importance = 4; // IMPORTANCE_HIGH

		jclass chanCls = find_class(env, "android/app/NotificationChannel");
		jmethodID chanCtor = find_method(env, chanCls, "<init>", "(Ljava/lang/String;Ljava/lang/CharSequence;I)V");
		jobject channel = (*env)->NewObject(env, chanCls, chanCtor, channelId, name, importance);

		jmethodID createChannel = find_method(env, mgrCls, "createNotificationChannel", "(Landroid/app/NotificationChannel;)V");
		(*env)->CallVoidMethod(env, mgr, createChannel, channel);

		jmethodID setChannelId = find_method(env, cls, "setChannelId", "(Ljava/lang/String;)Landroid/app/Notification$Builder;");
		(*env)->CallObjectMethod(env, builder, setChannelId, channelId);
	}

	jmethodID setContentTitle = find_method(env, cls, "setContentTitle", "(Ljava/lang/CharSequence;)Landroid/app/Notification$Builder;");
	(*env)->CallObjectMethod(env, builder, setContentTitle, titleStr);

	jmethodID setContentText = find_method(env, cls, "setContentText", "(Ljava/lang/CharSequence;)Landroid/app/Notification$Builder;");
	(*env)->CallObjectMethod(env, builder, setContentText, bodyStr);

	int iconID = 17629184; // constant of "unknown app icon"
	jmethodID setSmallIcon = find_method(env, cls, "setSmallIcon", "(I)Landroid/app/Notification$Builder;");
	(*env)->CallObjectMethod(env, builder, setSmallIcon, iconID);

	return builder;
}

// notifyBuilder builds the Notification and posts it, returning the id used.
static void notifyBuilder(JNIEnv *env, jobject ctx, jclass cls, jobject builder) {
	jclass mgrCls = find_class(env, "android/app/NotificationManager");
	jobject mgr = getSystemService((uintptr_t)env, (uintptr_t)ctx, "notification");

	jmethodID build = find_method(env, cls, "build", "()Landroid/app/Notification;");
	jobject notif = (*env)->CallObjectMethod(env, builder, build);

	jmethodID notify = find_method(env, mgrCls, "notify", "(ILandroid/app/Notification;)V");
	(*env)->CallVoidMethod(env, mgr, notify, nextId, notif);
	nextId++;
}

void sendNotification(uintptr_t java_vm, uintptr_t jni_env, uintptr_t ctx, char *title, char *body) {
	JNIEnv *env = (JNIEnv*)jni_env;
	ensureNotificationPermission(env, (jobject)ctx);
	jstring titleStr = (*env)->NewStringUTF(env, title);
	jstring bodyStr = (*env)->NewStringUTF(env, body);

	jclass cls = find_class(env, "android/app/Notification$Builder");
	jobject builder = buildBaseNotification(env, (jobject)ctx, cls, titleStr, bodyStr);
	notifyBuilder(env, (jobject)ctx, cls, builder);
}

int notifReqCode = 1;

// makeActivityPendingIntent builds a PendingIntent that re-delivers to the
// running GoNativeActivity (SINGLE_TOP -> onNewIntent) carrying the notification
// token and action index, so a tap routes back to Go via notificationActivated
// (fyne#723). action is -1 for the body tap, else the 0-based action index.
static jobject makeActivityPendingIntent(JNIEnv *env, jobject ctx, jclass actClass, jlong token, jint action) {
	jclass intentClass = find_class(env, "android/content/Intent");
	jmethodID intentCtor = find_method(env, intentClass, "<init>", "(Landroid/content/Context;Ljava/lang/Class;)V");
	jobject intent = (*env)->NewObject(env, intentClass, intentCtor, ctx, actClass);

	// FLAG_ACTIVITY_SINGLE_TOP (0x20000000): a running activity gets onNewIntent
	// rather than a new instance; FLAG_ACTIVITY_CLEAR_TOP (0x04000000) brings it forward.
	jmethodID setFlags = find_method(env, intentClass, "setFlags", "(I)Landroid/content/Intent;");
	(*env)->CallObjectMethod(env, intent, setFlags, (jint)(0x20000000 | 0x04000000));

	jstring tokenKey = (*env)->NewStringUTF(env, "bellui_notify_token");
	jmethodID putLong = find_method(env, intentClass, "putExtra", "(Ljava/lang/String;J)Landroid/content/Intent;");
	(*env)->CallObjectMethod(env, intent, putLong, tokenKey, token);

	jstring actionKey = (*env)->NewStringUTF(env, "bellui_notify_action");
	jmethodID putInt = find_method(env, intentClass, "putExtra", "(Ljava/lang/String;I)Landroid/content/Intent;");
	(*env)->CallObjectMethod(env, intent, putInt, actionKey, action);

	jclass piClass = find_class(env, "android/app/PendingIntent");
	jmethodID getActivity = find_static_method(env, piClass, "getActivity",
		"(Landroid/content/Context;ILandroid/content/Intent;I)Landroid/app/PendingIntent;");
	// FLAG_UPDATE_CURRENT (0x08000000) keeps one PendingIntent per (token,action);
	// FLAG_IMMUTABLE (0x04000000) is required on API 31+.
	jint flags = 0x08000000 | 0x04000000;
	jint req = notifReqCode++;
	return (*env)->CallStaticObjectMethod(env, piClass, getActivity, ctx, req, intent, flags);
}

// sendNotificationWithActions posts a rich notification (fyne#723): a body-tap
// content PendingIntent plus one PendingIntent per action button, each carrying
// the notification's token (and action index) back to Go. iconPath, when
// non-empty, is decoded as the large icon. labels is a labelCount-long array of
// action-button captions (in action-index order).
void sendNotificationWithActions(uintptr_t java_vm, uintptr_t jni_env, uintptr_t ctx_,
		long long token, char *title, char *body, char *iconPath, char **labels, int labelCount) {
	JNIEnv *env = (JNIEnv*)jni_env;
	jobject ctx = (jobject)ctx_;
	ensureNotificationPermission(env, ctx);
	jstring titleStr = (*env)->NewStringUTF(env, title);
	jstring bodyStr = (*env)->NewStringUTF(env, body);

	jclass cls = find_class(env, "android/app/Notification$Builder");
	jobject builder = buildBaseNotification(env, ctx, cls, titleStr, bodyStr);

	// Large icon from the temp file the Go side wrote, if any.
	if (iconPath != NULL && iconPath[0] != '\0') {
		jclass bfClass = find_class(env, "android/graphics/BitmapFactory");
		jmethodID decodeFile = find_static_method(env, bfClass, "decodeFile", "(Ljava/lang/String;)Landroid/graphics/Bitmap;");
		jstring pathStr = (*env)->NewStringUTF(env, iconPath);
		jobject bmp = (*env)->CallStaticObjectMethod(env, bfClass, decodeFile, pathStr);
		if (bmp != NULL) {
			jmethodID setLargeIcon = find_method(env, cls, "setLargeIcon", "(Landroid/graphics/Bitmap;)Landroid/app/Notification$Builder;");
			(*env)->CallObjectMethod(env, builder, setLargeIcon, bmp);
		}
	}

	// Dismiss the notification when the body is tapped.
	jmethodID setAutoCancel = find_method(env, cls, "setAutoCancel", "(Z)Landroid/app/Notification$Builder;");
	(*env)->CallObjectMethod(env, builder, setAutoCancel, JNI_TRUE);

	// The Intent target is GoNativeActivity; its class isn't reachable via
	// FindClass on a JVM-attached native thread, so resolve it from the ctx object.
	jclass actClass = (*env)->GetObjectClass(env, ctx);

	// Body tap -> action -1.
	jobject contentPI = makeActivityPendingIntent(env, ctx, actClass, (jlong)token, (jint)-1);
	jmethodID setContentIntent = find_method(env, cls, "setContentIntent", "(Landroid/app/PendingIntent;)Landroid/app/Notification$Builder;");
	(*env)->CallObjectMethod(env, builder, setContentIntent, contentPI);

	// Action buttons -> action index.
	jmethodID addAction = find_method(env, cls, "addAction", "(ILjava/lang/CharSequence;Landroid/app/PendingIntent;)Landroid/app/Notification$Builder;");
	for (int i = 0; i < labelCount; i++) {
		jstring labelStr = (*env)->NewStringUTF(env, labels[i]);
		jobject actionPI = makeActivityPendingIntent(env, ctx, actClass, (jlong)token, (jint)i);
		(*env)->CallObjectMethod(env, builder, addAction, (jint)0, labelStr, actionPI);
	}

	notifyBuilder(env, ctx, cls, builder);
}
