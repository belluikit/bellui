package org.golang.app;

import android.content.ContentProvider;
import android.content.ContentValues;
import android.database.Cursor;
import android.database.MatrixCursor;
import android.net.Uri;
import android.os.ParcelFileDescriptor;
import android.provider.OpenableColumns;
import android.util.Log;

import java.io.File;
import java.io.FileNotFoundException;
import java.io.IOException;

// BelluiFileProvider is a minimal ContentProvider that vends files Bellui has
// staged for sharing (fyne#711 ShareFile). The androidx FileProvider is not
// bundled in a bare gomobile build, so this stands in: it serves read-only
// files from a single, app-private directory (cacheDir/belluishare), the same
// directory the Go side copies the shared file into before firing ACTION_SEND.
//
// It is declared in the generated AndroidManifest with the authority
// "<applicationId>.fileprovider", exported=false and grantUriPermissions=true,
// so only recipients granted FLAG_GRANT_READ_URI_PERMISSION on the specific
// content:// URI can read the file.
public class BelluiFileProvider extends ContentProvider {
    private static final String SHARE_DIR = "belluishare";

    // uriForFile builds the content:// URI for a file previously placed in the
    // share directory. Only the file name is encoded into the path; the
    // provider resolves it back against its own share directory, so the
    // recipient never learns (or can traverse to) the real filesystem path.
    static Uri uriForFile(String authority, File file) {
        return new Uri.Builder()
                .scheme("content")
                .authority(authority)
                .appendPath(file.getName())
                .build();
    }

    private File resolve(Uri uri) throws FileNotFoundException {
        String name = uri.getLastPathSegment();
        if (name == null || name.isEmpty()) {
            throw new FileNotFoundException("empty path: " + uri);
        }
        File dir = new File(getContext().getCacheDir(), SHARE_DIR);
        File file = new File(dir, name);
        try {
            // Refuse anything that escapes the share directory (defence in depth;
            // the path is a single segment, so this should always hold).
            if (!file.getCanonicalPath().startsWith(dir.getCanonicalPath() + File.separator)) {
                throw new FileNotFoundException("outside share dir: " + uri);
            }
        } catch (IOException e) {
            throw new FileNotFoundException(e.getMessage());
        }
        return file;
    }

    @Override
    public boolean onCreate() {
        return true;
    }

    @Override
    public ParcelFileDescriptor openFile(Uri uri, String mode) throws FileNotFoundException {
        return ParcelFileDescriptor.open(resolve(uri), ParcelFileDescriptor.MODE_READ_ONLY);
    }

    @Override
    public Cursor query(Uri uri, String[] projection, String selection,
                        String[] selectionArgs, String sortOrder) {
        File file;
        try {
            file = resolve(uri);
        } catch (FileNotFoundException e) {
            Log.e("Bellui", "BelluiFileProvider query", e);
            return null;
        }
        String[] cols = {OpenableColumns.DISPLAY_NAME, OpenableColumns.SIZE};
        MatrixCursor cursor = new MatrixCursor(cols, 1);
        cursor.addRow(new Object[]{file.getName(), file.length()});
        return cursor;
    }

    @Override
    public String getType(Uri uri) {
        // The share Intent supplies the MIME type; recipients that resolve type
        // through the provider fall back to a generic stream.
        return "application/octet-stream";
    }

    @Override
    public Uri insert(Uri uri, ContentValues values) {
        throw new UnsupportedOperationException("read-only provider");
    }

    @Override
    public int delete(Uri uri, String selection, String[] selectionArgs) {
        throw new UnsupportedOperationException("read-only provider");
    }

    @Override
    public int update(Uri uri, ContentValues values, String selection, String[] selectionArgs) {
        throw new UnsupportedOperationException("read-only provider");
    }
}
