//go:build android

#include <android/log.h>
#include <dirent.h>
#include <jni.h>
#include <stdbool.h>
#include <stdlib.h>
#include <string.h>
#include <sys/stat.h>

#define LOG_FATAL(...) __android_log_print(ANDROID_LOG_FATAL, "Bellui", __VA_ARGS__)

// LOG_UNSUPPORTED reports an operation a scheme does not offer. It is not
// LOG_FATAL: an unsupported scheme is an ordinary error the Go caller turns
// into one, and logging it at FATAL priority reads in logcat as though the
// process were about to die. (Neither macro aborts — __android_log_print never
// does, whatever the priority; only __android_log_assert would.)
#define LOG_UNSUPPORTED(...) __android_log_print(ANDROID_LOG_WARN, "Bellui", __VA_ARGS__)

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

const char* getString(uintptr_t jni_env, uintptr_t ctx, jstring str) {
	JNIEnv *env = (JNIEnv*)jni_env;

	const char *chars = (*env)->GetStringUTFChars(env, str, NULL);

	const char *copy = strdup(chars);
	(*env)->ReleaseStringUTFChars(env, str, chars);
	return copy;
}

jobject parseURI(uintptr_t jni_env, uintptr_t ctx, char* uriCstr) {
	JNIEnv *env = (JNIEnv*)jni_env;

	jstring uriStr = (*env)->NewStringUTF(env, uriCstr);
	jclass uriClass = find_class(env, "android/net/Uri");
	jmethodID parse = find_static_method(env, uriClass, "parse", "(Ljava/lang/String;)Landroid/net/Uri;");

	return (jobject)(*env)->CallStaticObjectMethod(env, uriClass, parse, uriStr);
}

// clipboard

jobject getClipboard(uintptr_t jni_env, uintptr_t ctx) {
	JNIEnv *env = (JNIEnv*)jni_env;
	jclass ctxClass = (*env)->GetObjectClass(env, (jobject)ctx);
	jmethodID getSystemService = find_method(env, ctxClass, "getSystemService", "(Ljava/lang/String;)Ljava/lang/Object;");

	jstring service = (*env)->NewStringUTF(env, "clipboard");
	jobject ret = (*env)->CallObjectMethod(env, (jobject)ctx, getSystemService, service);
	jthrowable err = (*env)->ExceptionOccurred(env);

	if (err != NULL) {
		LOG_FATAL("cannot lookup clipboard");
		(*env)->ExceptionClear(env);
		return NULL;
	}
	return ret;
}

const char *getClipboardContent(uintptr_t java_vm, uintptr_t jni_env, uintptr_t ctx) {
	JNIEnv *env = (JNIEnv*)jni_env;
	jobject mgr = getClipboard(jni_env, ctx);
	if (mgr == NULL) {
		return NULL;
	}

	jclass mgrClass = (*env)->GetObjectClass(env, mgr);
	jmethodID getText = find_method(env, mgrClass, "getText", "()Ljava/lang/CharSequence;");

	jobject content = (jstring)(*env)->CallObjectMethod(env, mgr, getText);
	if (content == NULL) {
		return NULL;
	}

	jclass clzCharSequence = (*env)->GetObjectClass(env, content);
	jmethodID toString = (*env)->GetMethodID(env, clzCharSequence, "toString", "()Ljava/lang/String;");
	jobject s = (*env)->CallObjectMethod(env, content, toString);

	return getString(jni_env, ctx, s);
}

void setClipboardContent(uintptr_t java_vm, uintptr_t jni_env, uintptr_t ctx, char *content) {
	JNIEnv *env = (JNIEnv*)jni_env;
	jobject mgr = getClipboard(jni_env, ctx);
	if (mgr == NULL) {
		return;
	}

	jclass mgrClass = (*env)->GetObjectClass(env, mgr);
	jmethodID setText = find_method(env, mgrClass, "setText", "(Ljava/lang/CharSequence;)V");

	jstring str = (*env)->NewStringUTF(env, content);
	(*env)->CallVoidMethod(env, mgr, setText, str);
}

// file handling

jobject getContentResolver(uintptr_t jni_env, uintptr_t ctx) {
	JNIEnv *env = (JNIEnv*)jni_env;
	jclass ctxClass = (*env)->GetObjectClass(env, (jobject)ctx);
	jmethodID getContentResolver = find_method(env, ctxClass, "getContentResolver", "()Landroid/content/ContentResolver;");

	return (jobject)(*env)->CallObjectMethod(env, (jobject)ctx, getContentResolver);
}

void* openStream(uintptr_t jni_env, uintptr_t ctx, char* uriCstr) {
	JNIEnv *env = (JNIEnv*)jni_env;
	jobject resolver = getContentResolver(jni_env, ctx);

	jclass resolverClass = (*env)->GetObjectClass(env, resolver);
	jmethodID openInputStream = find_method(env, resolverClass, "openInputStream", "(Landroid/net/Uri;)Ljava/io/InputStream;");

	jobject uri = parseURI(jni_env, ctx, uriCstr);
	jobject stream = (jobject)(*env)->CallObjectMethod(env, resolver, openInputStream, uri);
	jthrowable loadErr = (*env)->ExceptionOccurred(env);

	if (loadErr != NULL) {
		(*env)->ExceptionClear(env);
		return NULL;
	}

	return (*env)->NewGlobalRef(env, stream);
}

void* saveStream(uintptr_t jni_env, uintptr_t ctx, char* uriCstr, bool truncate) {
	JNIEnv *env = (JNIEnv*)jni_env;
	jobject resolver = getContentResolver(jni_env, ctx);

	jclass resolverClass = (*env)->GetObjectClass(env, resolver);
	jmethodID saveOutputStream = find_method(env, resolverClass, "openOutputStream", "(Landroid/net/Uri;Ljava/lang/String;)Ljava/io/OutputStream;");

	jobject uri = parseURI(jni_env, ctx, uriCstr);
	jstring modes = NULL;
	if (truncate) {
		modes = (*env)->NewStringUTF(env, "wt"); // truncate before write
	} else {
		modes = (*env)->NewStringUTF(env, "wa");
	}
	jobject stream = (jobject)(*env)->CallObjectMethod(env, resolver, saveOutputStream, uri, modes);
	jthrowable loadErr = (*env)->ExceptionOccurred(env);

	if (loadErr != NULL) {
		(*env)->ExceptionClear(env);
		return NULL;
	}

	return (*env)->NewGlobalRef(env, stream);
}

jbyte* readStream(uintptr_t jni_env, uintptr_t ctx, void* stream, int len, int* total) {
	if (stream == NULL) {
		*total = -1;
		return NULL;
	}
	JNIEnv *env = (JNIEnv*)jni_env;
	jclass streamClass = (*env)->GetObjectClass(env, stream);
	jmethodID read = find_method(env, streamClass, "read", "([BII)I");

	jbyteArray data = (*env)->NewByteArray(env, len);
	int count = (int)(*env)->CallIntMethod(env, stream, read, data, 0, len);
	*total = count;

	if (count == -1) {
		return NULL;
	}

	jbyte* bytes = (jbyte*)malloc(sizeof(jbyte)*count);
	(*env)->GetByteArrayRegion(env, data, 0, count, bytes);
	return bytes;
}

void writeStream(uintptr_t jni_env, uintptr_t ctx, void* stream, jbyte* buf, int len) {
	if (stream == NULL) {
		if (buf != NULL) {
			free(buf);
		}
		return;
	}
	JNIEnv *env = (JNIEnv*)jni_env;
	jclass streamClass = (*env)->GetObjectClass(env, stream);
	jmethodID write = find_method(env, streamClass, "write", "([BII)V");

	jbyteArray data = (*env)->NewByteArray(env, len);
	(*env)->SetByteArrayRegion(env, data, 0, len, buf);

	(*env)->CallVoidMethod(env, stream, write, data, 0, len);

	free(buf);
}

void closeStream(uintptr_t jni_env, uintptr_t ctx, void* stream) {
	if (stream == NULL) {
		return;
	}
	JNIEnv *env = (JNIEnv*)jni_env;
	jclass streamClass = (*env)->GetObjectClass(env, stream);
	jmethodID close = find_method(env, streamClass, "close", "()V");
	(*env)->CallVoidMethod(env, stream, close);

	(*env)->DeleteGlobalRef(env, stream);
}

bool hasPrefix(char* string, char* prefix) {
	size_t lp = strlen(prefix);
	size_t ls = strlen(string);
	if (ls < lp) {
		return false;
	}
	return memcmp(prefix, string, lp) == 0;
}

// uriToString copies an android.net.Uri's string form onto the C heap; the Go
// caller frees it.
static const char* uriToString(uintptr_t jni_env, uintptr_t ctx, jobject uri) {
	JNIEnv *env = (JNIEnv*)jni_env;
	jclass uriClass = (*env)->GetObjectClass(env, uri);
	jmethodID toString = (*env)->GetMethodID(env, uriClass, "toString", "()Ljava/lang/String;");
	jstring s = (jstring)(*env)->CallObjectMethod(env, uri, toString);
	return getString(jni_env, ctx, s);
}

// docIDForURI answers the document ID a tree URI addresses: the document's own
// when the URI names a document under the tree, the tree root's otherwise.
//
// getTreeDocumentId always returns the ROOT's ID — it reads the /tree/<id>
// segment, which every URI built by buildDocumentUriUsingTree still carries.
// Using it for a sub-document is what made List() and CanList() answer for the
// picked folder no matter how deep the URI pointed (#303).
static jstring docIDForURI(uintptr_t jni_env, uintptr_t ctx, jclass contractClass, jobject uri) {
	JNIEnv *env = (JNIEnv*)jni_env;

	jboolean document = JNI_FALSE;
	jmethodID isDoc = find_static_method(env, contractClass, "isDocumentUri", "(Landroid/content/Context;Landroid/net/Uri;)Z");
	if (isDoc != 0) {
		document = (*env)->CallStaticBooleanMethod(env, contractClass, isDoc, (jobject)ctx, uri);
		if ((*env)->ExceptionOccurred(env) != NULL) {
			(*env)->ExceptionClear(env);
			document = JNI_FALSE;
		}
	}

	jmethodID getDoc = find_static_method(env, contractClass, document ? "getDocumentId" : "getTreeDocumentId", "(Landroid/net/Uri;)Ljava/lang/String;");
	if (getDoc == 0) { // API 21
		return NULL;
	}
	jstring docID = (jstring)(*env)->CallStaticObjectMethod(env, contractClass, getDoc, uri);
	if ((*env)->ExceptionOccurred(env) != NULL) { // not a tree URI
		(*env)->ExceptionClear(env);
		return NULL;
	}
	return docID;
}

// documentURI resolves a tree or document URI to the document URI the
// ContentResolver operates on. Returns NULL when uri is not under a tree the
// app holds a grant for.
static jobject documentURI(uintptr_t jni_env, uintptr_t ctx, jclass contractClass, jobject uri) {
	JNIEnv *env = (JNIEnv*)jni_env;

	jstring docID = docIDForURI(jni_env, ctx, contractClass, uri);
	if (docID == NULL) {
		return NULL;
	}
	jmethodID build = find_static_method(env, contractClass, "buildDocumentUriUsingTree", "(Landroid/net/Uri;Ljava/lang/String;)Landroid/net/Uri;");
	if (build == 0) {
		return NULL;
	}
	jobject doc = (jobject)(*env)->CallStaticObjectMethod(env, contractClass, build, uri, docID);
	if ((*env)->ExceptionOccurred(env) != NULL) {
		(*env)->ExceptionClear(env);
		return NULL;
	}
	return doc;
}

// queryChildren opens a cursor over the children of the document uri addresses,
// projecting document_id then _display_name. Returns NULL when uri is not a
// readable directory.
static jobject queryChildren(uintptr_t jni_env, uintptr_t ctx, jclass contractClass, jobject uri) {
	JNIEnv *env = (JNIEnv*)jni_env;
	jobject resolver = getContentResolver(jni_env, ctx);

	jstring docID = docIDForURI(jni_env, ctx, contractClass, uri);
	if (docID == NULL) {
		return NULL;
	}
	jmethodID getChild = find_static_method(env, contractClass, "buildChildDocumentsUriUsingTree", "(Landroid/net/Uri;Ljava/lang/String;)Landroid/net/Uri;");
	if (getChild == 0) {
		return NULL;
	}
	jobject childrenUri = (jobject)(*env)->CallStaticObjectMethod(env, contractClass, getChild, uri, docID);
	if ((*env)->ExceptionOccurred(env) != NULL) {
		(*env)->ExceptionClear(env);
		return NULL;
	}

	jclass stringClass = find_class(env, "java/lang/String");
	jobjectArray project = (*env)->NewObjectArray(env, 2, stringClass, (*env)->NewStringUTF(env, "document_id"));
	(*env)->SetObjectArrayElement(env, project, 1, (*env)->NewStringUTF(env, "_display_name"));

	jclass resolverClass = (*env)->GetObjectClass(env, resolver);
	jmethodID query = find_method(env, resolverClass, "query", "(Landroid/net/Uri;[Ljava/lang/String;Landroid/os/Bundle;Landroid/os/CancellationSignal;)Landroid/database/Cursor;");
	if (query == NULL) { // API 26
		return NULL;
	}
	jobject cursor = (jobject)(*env)->CallObjectMethod(env, resolver, query, childrenUri, project, NULL, NULL);
	if ((*env)->ExceptionOccurred(env) != NULL) { // no grant, or not a directory
		(*env)->ExceptionClear(env);
		return NULL;
	}
	return cursor;
}

static void closeCursor(JNIEnv *env, jobject cursor) {
	jclass cursorClass = (*env)->GetObjectClass(env, cursor);
	jmethodID close = find_method(env, cursorClass, "close", "()V");
	(*env)->CallVoidMethod(env, cursor, close);
}

// findChildDocumentURI returns the document URI of uri's child whose display
// name is nameCstr, or NULL when there is none. SAF has no path arithmetic —
// document IDs are opaque — so the only way to name a child is to ask the
// provider which of its children carries that display name.
static jobject findChildDocumentURI(uintptr_t jni_env, uintptr_t ctx, jclass contractClass, jobject uri, char* nameCstr) {
	JNIEnv *env = (JNIEnv*)jni_env;

	jobject cursor = queryChildren(jni_env, ctx, contractClass, uri);
	if (cursor == NULL) {
		return NULL;
	}

	jclass cursorClass = (*env)->GetObjectClass(env, cursor);
	jmethodID next = find_method(env, cursorClass, "moveToNext", "()Z");
	jmethodID get = find_method(env, cursorClass, "getString", "(I)Ljava/lang/String;");
	jmethodID build = find_static_method(env, contractClass, "buildDocumentUriUsingTree", "(Landroid/net/Uri;Ljava/lang/String;)Landroid/net/Uri;");

	jobject found = NULL;
	while (found == NULL && ((jboolean)(*env)->CallBooleanMethod(env, cursor, next)) == JNI_TRUE) {
		jstring childName = (jstring)(*env)->CallObjectMethod(env, cursor, get, 1);
		if (childName == NULL) {
			continue;
		}
		const char *name = getString(jni_env, ctx, childName);
		if (strcmp(name, nameCstr) == 0) {
			jstring childDocID = (jstring)(*env)->CallObjectMethod(env, cursor, get, 0);
			found = (jobject)(*env)->CallStaticObjectMethod(env, contractClass, build, uri, childDocID);
			if ((*env)->ExceptionOccurred(env) != NULL) {
				(*env)->ExceptionClear(env);
				found = NULL;
			}
			(*env)->DeleteLocalRef(env, childDocID);
		}
		free((void*)name);
		(*env)->DeleteLocalRef(env, childName);
	}

	closeCursor(env, cursor);
	return found;
}

// childContentURI implements Child() for content:// — the document URI of the
// child of uriCstr named nameCstr. NULL when no such child exists; SAF cannot
// name one that does not, which is what createChildContentURI is for.
char* childContentURI(uintptr_t jni_env, uintptr_t ctx, char* uriCstr, char* nameCstr) {
	JNIEnv *env = (JNIEnv*)jni_env;
	jobject uri = parseURI(jni_env, ctx, uriCstr);
	if ((*env)->ExceptionOccurred(env) != NULL) {
		(*env)->ExceptionClear(env);
		return NULL;
	}
	jclass contractClass = find_class(env, "android/provider/DocumentsContract");
	if (contractClass == NULL) { // API 19
		return NULL;
	}

	jobject child = findChildDocumentURI(jni_env, ctx, contractClass, uri, nameCstr);
	if (child == NULL) {
		return NULL;
	}
	return (char*)uriToString(jni_env, ctx, child);
}

// createChildContentURI creates a document named nameCstr of type mimeCstr
// under the directory uriCstr addresses and returns the URI the provider
// assigned it. mimeCstr is "vnd.android.document/directory" for a directory.
//
// It refuses when a child of that name is already there: providers answer
// createDocument by inventing "name (1)" instead of failing, and silently
// creating a differently-named document is not what a caller asking for
// "name" wants (mkdir reports EEXIST).
char* createChildContentURI(uintptr_t jni_env, uintptr_t ctx, char* uriCstr, char* mimeCstr, char* nameCstr) {
	JNIEnv *env = (JNIEnv*)jni_env;
	jobject resolver = getContentResolver(jni_env, ctx);
	jobject uri = parseURI(jni_env, ctx, uriCstr);
	if ((*env)->ExceptionOccurred(env) != NULL) {
		(*env)->ExceptionClear(env);
		return NULL;
	}
	jclass contractClass = find_class(env, "android/provider/DocumentsContract");
	if (contractClass == NULL) { // API 19
		return NULL;
	}

	if (findChildDocumentURI(jni_env, ctx, contractClass, uri, nameCstr) != NULL) {
		return NULL; // already exists
	}

	jobject parentDoc = documentURI(jni_env, ctx, contractClass, uri);
	if (parentDoc == NULL) {
		return NULL;
	}

	jmethodID create = find_static_method(env, contractClass, "createDocument",
		"(Landroid/content/ContentResolver;Landroid/net/Uri;Ljava/lang/String;Ljava/lang/String;)Landroid/net/Uri;");
	if (create == 0) { // API 21
		return NULL;
	}
	jstring mime = (*env)->NewStringUTF(env, mimeCstr);
	jstring name = (*env)->NewStringUTF(env, nameCstr);
	jobject child = (jobject)(*env)->CallStaticObjectMethod(env, contractClass, create, resolver, parentDoc, mime, name);
	if ((*env)->ExceptionOccurred(env) != NULL) {
		(*env)->ExceptionClear(env);
		return NULL;
	}
	if (child == NULL) {
		return NULL;
	}
	return (char*)uriToString(jni_env, ctx, child);
}

// existsContentURI reports whether the document uriCstr addresses is still
// there: a one-row query against the document itself, which is what the
// provider answers for a live document and refuses for a deleted one.
static bool existsContentURI(uintptr_t jni_env, uintptr_t ctx, char* uriCstr) {
	JNIEnv *env = (JNIEnv*)jni_env;
	jobject resolver = getContentResolver(jni_env, ctx);
	jobject uri = parseURI(jni_env, ctx, uriCstr);
	if ((*env)->ExceptionOccurred(env) != NULL) {
		(*env)->ExceptionClear(env);
		return false;
	}
	jclass contractClass = find_class(env, "android/provider/DocumentsContract");
	if (contractClass == NULL) { // API 19
		return false;
	}
	jobject doc = documentURI(jni_env, ctx, contractClass, uri);
	if (doc == NULL) {
		return false;
	}

	jclass stringClass = find_class(env, "java/lang/String");
	jobjectArray project = (*env)->NewObjectArray(env, 1, stringClass, (*env)->NewStringUTF(env, "document_id"));

	jclass resolverClass = (*env)->GetObjectClass(env, resolver);
	jmethodID query = find_method(env, resolverClass, "query", "(Landroid/net/Uri;[Ljava/lang/String;Landroid/os/Bundle;Landroid/os/CancellationSignal;)Landroid/database/Cursor;");
	if (query == NULL) { // API 26
		return false;
	}
	jobject cursor = (jobject)(*env)->CallObjectMethod(env, resolver, query, doc, project, NULL, NULL);
	if ((*env)->ExceptionOccurred(env) != NULL) {
		(*env)->ExceptionClear(env);
		return false;
	}
	if (cursor == NULL) {
		return false;
	}

	jclass cursorClass = (*env)->GetObjectClass(env, cursor);
	jmethodID first = find_method(env, cursorClass, "moveToFirst", "()Z");
	bool exists = ((jboolean)(*env)->CallBooleanMethod(env, cursor, first)) == JNI_TRUE;
	closeCursor(env, cursor);
	return exists;
}

// deleteContentURI removes the document uriCstr addresses, children included —
// DocumentsContract.deleteDocument is recursive for a directory.
static bool deleteContentURI(uintptr_t jni_env, uintptr_t ctx, char* uriCstr) {
	JNIEnv *env = (JNIEnv*)jni_env;
	jobject resolver = getContentResolver(jni_env, ctx);
	jobject uri = parseURI(jni_env, ctx, uriCstr);
	if ((*env)->ExceptionOccurred(env) != NULL) {
		(*env)->ExceptionClear(env);
		return false;
	}
	jclass contractClass = find_class(env, "android/provider/DocumentsContract");
	if (contractClass == NULL) { // API 19
		return false;
	}
	jobject doc = documentURI(jni_env, ctx, contractClass, uri);
	if (doc == NULL) {
		return false;
	}

	jmethodID del = find_static_method(env, contractClass, "deleteDocument", "(Landroid/content/ContentResolver;Landroid/net/Uri;)Z");
	if (del == 0) { // API 19
		return false;
	}
	jboolean ok = (*env)->CallStaticBooleanMethod(env, contractClass, del, resolver, doc);
	if ((*env)->ExceptionOccurred(env) != NULL) { // no write grant
		(*env)->ExceptionClear(env);
		return false;
	}
	return ok == JNI_TRUE;
}

bool canListContentURI(uintptr_t jni_env, uintptr_t ctx, char* uriCstr) {
	JNIEnv *env = (JNIEnv*)jni_env;
	jobject resolver = getContentResolver(jni_env, ctx);
	jobject uri = parseURI(jni_env, ctx, uriCstr);
	jthrowable loadErr = (*env)->ExceptionOccurred(env);

	if (loadErr != NULL) {
		(*env)->ExceptionClear(env);
		return false;
	}

	jclass contractClass = find_class(env, "android/provider/DocumentsContract");
	if (contractClass == NULL) { // API 19
		return false;
	}

	jobject doc = documentURI(jni_env, ctx, contractClass, uri);
	if (doc == NULL) { // not a tree URI, or API 21
		return false;
	}

	jclass resolverClass = (*env)->GetObjectClass(env, resolver);
	jmethodID getType = find_method(env, resolverClass, "getType", "(Landroid/net/Uri;)Ljava/lang/String;");
	jstring type = (jstring)(*env)->CallObjectMethod(env, resolver, getType, doc);
	if ((*env)->ExceptionOccurred(env) != NULL) {
		(*env)->ExceptionClear(env);
		type = NULL;
	}

	if (type != NULL) {
		const char *str = getString(jni_env, ctx, type);
		int dir = strcmp(str, "vnd.android.document/directory") == 0;
		free((void*)str);
		if (dir) {
			return true;
		}
	}

	// Some providers answer getType with null or a non-directory type for a
	// tree root the app holds a valid grant for — the downloads provider
	// behind the picker's Download shortcut does (#244). The children query
	// is the operation List() actually performs (listContentURI below), so
	// attempt it: getting a cursor back, even an empty one, proves the URI
	// is listable.
	jobject cursor = queryChildren(jni_env, ctx, contractClass, uri);
	if (cursor == NULL) {
		return false;
	}

	closeCursor(env, cursor);
	return true;
}

bool canListFileURI(char* uriCstr) {
	// Get file path from URI
	size_t length = strlen(uriCstr)-7;// -7 for 'file://'
	char* path = malloc(sizeof(char)*(length+1));// +1 for '\0'
	memcpy(path, &uriCstr[7], length);
	path[length] = '\0';

	// Stat path to determine if it points to a directory
	struct stat statbuf;
	int result = stat(path, &statbuf);

	free(path);

	return (result == 0) && S_ISDIR(statbuf.st_mode);
}

bool canListURI(uintptr_t jni_env, uintptr_t ctx, char* uriCstr) {
	if (hasPrefix(uriCstr, "file://")) {
		return canListFileURI(uriCstr);
	} else if (hasPrefix(uriCstr, "content://")) {
		return canListContentURI(jni_env, ctx, uriCstr);
	}
	LOG_UNSUPPORTED("Unrecognized scheme: %s", uriCstr);
	return false;
}

bool createListableFileURI(char* uriCstr) {
	// Get file path from URI
	size_t length = strlen(uriCstr)-7;// -7 for 'file://'
	char* path = malloc(sizeof(char)*(length+1));// +1 for '\0'
	memcpy(path, &uriCstr[7], length);
	path[length] = '\0';

	int result = mkdir(path, S_IRWXU);
	free(path);

	return result == 0;
}

// createListableURI creates the directory uriCstr names. It stays file:// only:
// under a SAF tree a directory cannot be named before it exists, because the
// provider assigns the document ID, so the content:// path is CreateChild
// (createChildContentURI above) rather than this one (#303).
bool createListableURI(uintptr_t jni_env, uintptr_t ctx, char* uriCstr) {
	if (hasPrefix(uriCstr, "file://")) {
		return createListableFileURI(uriCstr);
	}
	LOG_UNSUPPORTED("Cannot create directory for scheme: %s — use storage.CreateChild", uriCstr);
	return false;
}

const char* contentURIGetFileName(uintptr_t jni_env, uintptr_t ctx, char* uriCstr) {
	JNIEnv *env = (JNIEnv*)jni_env;
	jobject resolver = getContentResolver(jni_env, ctx);
	jobject uri = parseURI(jni_env, ctx, uriCstr);
	jthrowable loadErr = (*env)->ExceptionOccurred(env);

	if (loadErr != NULL) {
		(*env)->ExceptionClear(env);
		return "";
	}

	jclass stringClass = find_class(env, "java/lang/String");
	jobjectArray project = (*env)->NewObjectArray(env, 1, stringClass, (*env)->NewStringUTF(env, "_display_name"));

	jclass resolverClass = (*env)->GetObjectClass(env, resolver);
	jmethodID query = find_method(env, resolverClass, "query", "(Landroid/net/Uri;[Ljava/lang/String;Ljava/lang/String;[Ljava/lang/String;Ljava/lang/String;)Landroid/database/Cursor;");

	jobject cursor = (jobject)(*env)->CallObjectMethod(env, resolver, query, uri, project, NULL, NULL, NULL);
	jclass cursorClass = (*env)->GetObjectClass(env, cursor);

	jmethodID first = find_method(env, cursorClass, "moveToFirst", "()Z");
	jmethodID get = find_method(env, cursorClass, "getString", "(I)Ljava/lang/String;");

	if (((jboolean)(*env)->CallBooleanMethod(env, cursor, first)) == JNI_TRUE) {
		jstring name = (jstring)(*env)->CallObjectMethod(env, cursor, get, 0);
		const char *fname = getString(jni_env, ctx, name);
		return fname;
	}

	return NULL;
}

char *filePath(char *uriCstr) {
	// Get file path from URI
	size_t length = strlen(uriCstr)-7;// -7 for 'file://'
	char* path = malloc(sizeof(char)*(length+1));// +1 for '\0'
	memcpy(path, &uriCstr[7], length);
	path[length] = '\0';

	return path;
}

bool deleteFileURI(char *uriCstr) {
	char* path = filePath(uriCstr);
	int result = remove(path);

	free(path);

	return result == 0;
}

bool deleteURI(uintptr_t jni_env, uintptr_t ctx, char* uriCstr) {
	if (hasPrefix(uriCstr, "file://")) {
		return deleteFileURI(uriCstr);
	} else if (hasPrefix(uriCstr, "content://")) {
		return deleteContentURI(jni_env, ctx, uriCstr);
	}
	LOG_UNSUPPORTED("Cannot delete for scheme: %s", uriCstr);
	return false;
}

bool existsFileURI(char* uriCstr) {
	char* path = filePath(uriCstr);

	// Stat path to determine if it points to an existing file
	struct stat statbuf;
	int result = stat(path, &statbuf);

	free(path);

	return result == 0;
}

bool existsURI(uintptr_t jni_env, uintptr_t ctx, char* uriCstr) {
	if (hasPrefix(uriCstr, "file://")) {
		return existsFileURI(uriCstr);
	} else if (hasPrefix(uriCstr, "content://")) {
		return existsContentURI(jni_env, ctx, uriCstr);
	}
	LOG_UNSUPPORTED("Cannot check exists for scheme: %s", uriCstr);
	return false;
}

char* listContentURI(uintptr_t jni_env, uintptr_t ctx, char* uriCstr) {
	JNIEnv *env = (JNIEnv*)jni_env;
	jobject resolver = getContentResolver(jni_env, ctx);
	jobject uri = parseURI(jni_env, ctx, uriCstr);
	jthrowable loadErr = (*env)->ExceptionOccurred(env);

	if (loadErr != NULL) {
		(*env)->ExceptionClear(env);
		return "";
	}

	jclass contractClass = find_class(env, "android/provider/DocumentsContract");
	if (contractClass == NULL) { // API 19
		return "";
	}

	jobject cursor = queryChildren(jni_env, ctx, contractClass, uri);
	if (cursor == NULL) {
		return "";
	}

	jclass cursorClass = (*env)->GetObjectClass(env, cursor);
	jmethodID next = find_method(env, cursorClass, "moveToNext", "()Z");
	jmethodID get = find_method(env, cursorClass, "getString", "(I)Ljava/lang/String;");
	jmethodID getChildURI = find_static_method(env, contractClass, "buildDocumentUriUsingTree", "(Landroid/net/Uri;Ljava/lang/String;)Landroid/net/Uri;");

	char *ret = NULL;
	int len = 0;
	while (((jboolean)(*env)->CallBooleanMethod(env, cursor, next)) == JNI_TRUE) {
		jstring childDocId = (jstring)(*env)->CallObjectMethod(env, cursor, get, 0);
		jobject childUri = (jobject)(*env)->CallStaticObjectMethod(env, contractClass, getChildURI, uri, childDocId);

		const char *uid = uriToString(jni_env, ctx, childUri);

		// append
		char *old = ret;
		len = len + strlen(uid) + 1;
		ret = malloc(sizeof(char)*(len+1));
		if (old != NULL) {
			strcpy(ret, old);
			free(old);
		} else {
			ret[0] = '\0';
		}
		strcat(ret, uid);
		strcat(ret, "|");

		// A directory of any size otherwise overruns the JNI local reference
		// table (512 entries by default), which aborts the process.
		free((void*)uid);
		(*env)->DeleteLocalRef(env, childUri);
		(*env)->DeleteLocalRef(env, childDocId);
	}

	closeCursor(env, cursor);

	if (ret != NULL) {
		ret[len-1] = '\0';
	}
	return ret;
}

char* listFileURI(char* uriCstr) {

	size_t uriLength = strlen(uriCstr);

	// Get file path from URI
	size_t length = uriLength-7;// -7 for 'file://'
	char* path = malloc(sizeof(char)*(length+1));// +1 for '\0'
	memcpy(path, &uriCstr[7], length);
	path[length] = '\0';

	char *ret = NULL;
	DIR *dfd;
	if ((dfd = opendir(path)) != NULL) {
		struct dirent *dp;
		int len = 0;
		while ((dp = readdir(dfd)) != NULL) {
			if (strcmp(dp->d_name, ".") == 0) {
				// Ignore current directory
				continue;
			}
			if (strcmp(dp->d_name, "..") == 0) {
				// Ignore parent directory
				continue;
			}
			// append
			char *old = ret;
			len = len + uriLength + 1 /* / */ + strlen(dp->d_name) + 1 /* | */;
			ret = malloc(sizeof(char)*(len+1));
			if (old != NULL) {
				strcpy(ret, old);
				free(old);
			} else {
				ret[0] = '\0';
			}
			strcat(ret, uriCstr);
			strcat(ret, "/");
			strcat(ret, dp->d_name);
			strcat(ret, "|");
		}
		if (ret != NULL) {
			ret[len-1] = '\0';
		}
	}

	free(path);

	return ret;
}

char* listURI(uintptr_t jni_env, uintptr_t ctx, char* uriCstr) {
	if (hasPrefix(uriCstr, "file://")) {
		return listFileURI(uriCstr);
	} else if (hasPrefix(uriCstr, "content://")) {
		return listContentURI(jni_env, ctx, uriCstr);
	}
	LOG_UNSUPPORTED("Unrecognized scheme: %s", uriCstr);
	return "";
}

void keepScreenOn(uintptr_t jni_env, uintptr_t ctx, bool disabled) {
	JNIEnv *env = (JNIEnv*)jni_env;
	jclass activityClass = find_class(env, "android/app/Activity");
	jmethodID getWindow = find_method(env, activityClass, "getWindow", "()Landroid/view/Window;");

	jobject win = (*env)->CallObjectMethod(env, (jobject)ctx, getWindow);
	jclass windowClass = find_class(env, "android/view/Window");

	jmethodID action = NULL;
	if (disabled) {
	    action = find_method(env, windowClass, "addFlags", "(I)V");
	} else {
	    action = find_method(env, windowClass, "clearFlags", "(I)V");
	}

    jclass paramsClass = find_class(env, "android/view/WindowManager$LayoutParams" );
    jfieldID screenFlagField = (*env)->GetStaticFieldID(env, paramsClass, "FLAG_KEEP_SCREEN_ON", "I" );
    int screenFlag = (*env)->GetStaticIntField(env, paramsClass, screenFlagField);

   	(*env)->CallVoidMethod(env, win, action, screenFlag);
}
