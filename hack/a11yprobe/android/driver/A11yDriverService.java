package com.trendvidia.a11ydriver;

import android.accessibilityservice.AccessibilityService;
import android.content.BroadcastReceiver;
import android.content.Context;
import android.content.Intent;
import android.content.IntentFilter;
import android.os.Build;
import android.os.Bundle;
import android.util.Log;
import android.view.accessibility.AccessibilityEvent;
import android.view.accessibility.AccessibilityNodeInfo;

/**
 * Adb-driven accessibility client for exercising a11y actions that no
 * TalkBack gesture can send (the "UIA-style test service" of fyne#951).
 *
 * Enable (append to any existing services, colon-separated):
 *   adb shell settings put secure enabled_accessibility_services \
 *       com.trendvidia.a11ydriver/com.trendvidia.a11ydriver.A11yDriverService
 *   adb shell settings put secure accessibility_enabled 1
 *
 * Drive (target app must be foreground):
 *   adb shell am broadcast -a com.trendvidia.a11ydriver.DUMP
 *   adb shell am broadcast -a com.trendvidia.a11ydriver.SCROLLTO --ei row 400 --ei col 0
 *   adb shell am broadcast -a com.trendvidia.a11ydriver.FOCUS --ei row 400 --ei col 0
 *
 * Watch:  adb logcat -s BelluiA11yDriver
 */
public class A11yDriverService extends AccessibilityService {
    static final String TAG = "BelluiA11yDriver";
    private BroadcastReceiver receiver;

    @Override
    public void onAccessibilityEvent(AccessibilityEvent event) {}

    @Override
    public void onInterrupt() {}

    @Override
    protected void onServiceConnected() {
        super.onServiceConnected();
        receiver = new BroadcastReceiver() {
            @Override
            public void onReceive(Context ctx, Intent intent) {
                try {
                    handle(intent);
                } catch (Throwable t) {
                    Log.e(TAG, "handle failed", t);
                }
            }
        };
        IntentFilter f = new IntentFilter();
        f.addAction("com.trendvidia.a11ydriver.SCROLLTO");
        f.addAction("com.trendvidia.a11ydriver.FOCUS");
        f.addAction("com.trendvidia.a11ydriver.DUMP");
        f.addAction("com.trendvidia.a11ydriver.ACT");
        if (Build.VERSION.SDK_INT >= 33) {
            registerReceiver(receiver, f, Context.RECEIVER_EXPORTED);
        } else {
            registerReceiver(receiver, f);
        }
        Log.i(TAG, "connected; broadcast SCROLLTO/FOCUS (--ei row N --ei col M) or DUMP");
    }

    @Override
    public void onDestroy() {
        if (receiver != null) unregisterReceiver(receiver);
        super.onDestroy();
    }

    void handle(Intent intent) {
        AccessibilityNodeInfo root = getRootInActiveWindow();
        if (root == null) {
            Log.e(TAG, "no active window root");
            return;
        }
        String act = intent.getAction();
        if ("com.trendvidia.a11ydriver.DUMP".equals(act)) {
            dump(root, 0);
            return;
        }
        if ("com.trendvidia.a11ydriver.ACT".equals(act)) {
            // am broadcast -a com.trendvidia.a11ydriver.ACT --es text "West (125)" --es action expand|collapse|click|focus
            String text = intent.getStringExtra("text");
            String name = intent.getStringExtra("action");
            AccessibilityNodeInfo node = findByText(root, text);
            if (node == null) {
                Log.e(TAG, "no node with text containing [" + text + "]");
                return;
            }
            int id;
            if ("expand".equals(name)) id = AccessibilityNodeInfo.ACTION_EXPAND;
            else if ("collapse".equals(name)) id = AccessibilityNodeInfo.ACTION_COLLAPSE;
            else if ("click".equals(name)) id = AccessibilityNodeInfo.ACTION_CLICK;
            else if ("focus".equals(name)) id = AccessibilityNodeInfo.ACTION_ACCESSIBILITY_FOCUS;
            else {
                // Custom action: match by label substring (case-insensitive),
                // the way a TalkBack user picks it from the actions menu.
                id = 0;
                for (AccessibilityNodeInfo.AccessibilityAction a : node.getActionList()) {
                    if (a.getLabel() != null
                            && a.getLabel().toString().toLowerCase().contains(name.toLowerCase())) {
                        id = a.getId();
                        break;
                    }
                }
                if (id == 0) { Log.e(TAG, "no action labeled [" + name + "]"); return; }
            }
            Log.i(TAG, "node [" + node.getText() + "] class=" + node.getClassName()
                    + " actions=" + node.getActionList());
            boolean ok = node.performAction(id);
            Log.i(TAG, name + " on [" + node.getText() + "] -> " + ok);
            return;
        }
        AccessibilityNodeInfo grid = findGrid(root);
        if (grid == null) {
            Log.e(TAG, "no grid (CollectionInfo rowCount>1) node in active window");
            return;
        }
        AccessibilityNodeInfo.CollectionInfo ci = grid.getCollectionInfo();
        Log.i(TAG, "grid: rows=" + ci.getRowCount() + " cols=" + ci.getColumnCount()
                + " class=" + grid.getClassName() + " actions=" + grid.getActionList());
        int row = intent.getIntExtra("row", 0);
        int col = intent.getIntExtra("col", 0);
        if ("com.trendvidia.a11ydriver.SCROLLTO".equals(act)) {
            Bundle args = new Bundle();
            args.putInt(AccessibilityNodeInfo.ACTION_ARGUMENT_ROW_INT, row);
            args.putInt(AccessibilityNodeInfo.ACTION_ARGUMENT_COLUMN_INT, col);
            boolean ok = grid.performAction(
                    AccessibilityNodeInfo.AccessibilityAction.ACTION_SCROLL_TO_POSITION.getId(), args);
            Log.i(TAG, "SCROLL_TO_POSITION row=" + row + " col=" + col + " -> " + ok);
        } else if ("com.trendvidia.a11ydriver.FOCUS".equals(act)) {
            AccessibilityNodeInfo cell = findCell(grid, row, col);
            if (cell == null) {
                Log.e(TAG, "no realized cell at row=" + row + " col=" + col);
                return;
            }
            boolean ok = cell.performAction(AccessibilityNodeInfo.ACTION_ACCESSIBILITY_FOCUS);
            Log.i(TAG, "ACCESSIBILITY_FOCUS row=" + row + " col=" + col
                    + " text=" + cell.getText() + " -> " + ok);
        }
    }

    AccessibilityNodeInfo findGrid(AccessibilityNodeInfo n) {
        if (n == null) return null;
        AccessibilityNodeInfo.CollectionInfo ci = n.getCollectionInfo();
        if (ci != null && ci.getRowCount() > 1) return n;
        for (int i = 0; i < n.getChildCount(); i++) {
            AccessibilityNodeInfo r = findGrid(n.getChild(i));
            if (r != null) return r;
        }
        return null;
    }

    AccessibilityNodeInfo findByText(AccessibilityNodeInfo n, String text) {
        if (n == null || text == null) return null;
        if (n.getText() != null && n.getText().toString().contains(text)) return n;
        for (int i = 0; i < n.getChildCount(); i++) {
            AccessibilityNodeInfo r = findByText(n.getChild(i), text);
            if (r != null) return r;
        }
        return null;
    }

    AccessibilityNodeInfo findCell(AccessibilityNodeInfo n, int row, int col) {
        if (n == null) return null;
        AccessibilityNodeInfo.CollectionItemInfo ii = n.getCollectionItemInfo();
        if (ii != null && ii.getRowIndex() == row && ii.getColumnIndex() == col) return n;
        for (int i = 0; i < n.getChildCount(); i++) {
            AccessibilityNodeInfo r = findCell(n.getChild(i), row, col);
            if (r != null) return r;
        }
        return null;
    }

    void dump(AccessibilityNodeInfo n, int depth) {
        if (n == null) return;
        StringBuilder sb = new StringBuilder();
        for (int i = 0; i < depth; i++) sb.append("  ");
        sb.append(n.getClassName()).append(" text=").append(n.getText());
        android.graphics.Rect r = new android.graphics.Rect();
        n.getBoundsInScreen(r);
        sb.append(" ").append(r.toShortString());
        AccessibilityNodeInfo.CollectionInfo ci = n.getCollectionInfo();
        if (ci != null) sb.append(" [collection ").append(ci.getRowCount()).append("x").append(ci.getColumnCount()).append("]");
        AccessibilityNodeInfo.CollectionItemInfo ii = n.getCollectionItemInfo();
        if (ii != null) sb.append(" [item r").append(ii.getRowIndex()).append(" c").append(ii.getColumnIndex()).append("]");
        if (n.isScrollable()) sb.append(" scrollable");
        Log.i(TAG, sb.toString());
        for (int i = 0; i < n.getChildCount(); i++) dump(n.getChild(i), depth + 1);
    }
}
