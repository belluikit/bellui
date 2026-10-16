//go:build (linux && !wayland && !wasm && !test_web_driver) || freebsd || openbsd || netbsd

// XDND (X Drag-and-Drop) *source* protocol, hand-rolled for drag-out (pre-rebrand tracker #838,
// pre-rebrand tracker #515 Part B). GLFW exposes no drag-source API and keeps its Xlib Display on
// its own thread, so rather than fight GLFW's event loop we run the whole drag
// on a *private* Xlib connection (opened here, used only by the caller's locked
// OS thread — see dragsource_x11.go). That connection owns the XdndSelection,
// so every SelectionRequest and every XdndStatus/XdndFinished ClientMessage the
// target sends back to our source window is delivered to *our* queue; nothing
// is stolen from or shared with GLFW. We track the pointer by polling
// XQueryPointer (no active grab — X reports global pointer + button state to
// any client), which side-steps the implicit button grab GLFW's window holds
// during the drag. See https://freedesktop.org/wiki/Specifications/XDND/.

#ifndef _POSIX_C_SOURCE
#define _POSIX_C_SOURCE 199309L // nanosleep, clock_gettime, CLOCK_MONOTONIC
#endif

#include "dragsource_x11.h"

#include <X11/Xatom.h>
#include <X11/Xlib.h>
#include <X11/Xutil.h>
#include <X11/extensions/shape.h>
#include <stdlib.h>
#include <string.h>
#include <time.h>

// BELLUI_XDND_VERSION is the XDND protocol version we implement as a source
// (v5 is the current, widely-deployed revision).
#define BELLUI_XDND_VERSION 5

// XdndAtoms caches the interned protocol atoms for one drag.
typedef struct {
	Atom aware, selection, enter, position, status, leave, drop, finished, typelist;
	Atom actionCopy, targets;
} XdndAtoms;

static void internAtoms(Display* dpy, XdndAtoms* a) {
	a->aware = XInternAtom(dpy, "XdndAware", False);
	a->selection = XInternAtom(dpy, "XdndSelection", False);
	a->enter = XInternAtom(dpy, "XdndEnter", False);
	a->position = XInternAtom(dpy, "XdndPosition", False);
	a->status = XInternAtom(dpy, "XdndStatus", False);
	a->leave = XInternAtom(dpy, "XdndLeave", False);
	a->drop = XInternAtom(dpy, "XdndDrop", False);
	a->finished = XInternAtom(dpy, "XdndFinished", False);
	a->typelist = XInternAtom(dpy, "XdndTypeList", False);
	a->actionCopy = XInternAtom(dpy, "XdndActionCopy", False);
	a->targets = XInternAtom(dpy, "TARGETS", False);
}

// napSleep yields the CPU for ms milliseconds between pointer samples so the
// poll loop is not a busy spin.
static void napSleep(int ms) {
	struct timespec ts;
	ts.tv_sec = ms / 1000;
	ts.tv_nsec = (long)(ms % 1000) * 1000000L;
	nanosleep(&ts, NULL);
}

// xdndAwareVersion returns the XDND protocol version advertised on win via its
// XdndAware property, or 0 when the window is not XDND-aware. The version is the
// low byte of the first 32-bit element of the property.
static int xdndAwareVersion(Display* dpy, Window win, Atom awareAtom) {
	Atom type = None;
	int fmt = 0;
	unsigned long nitems = 0, after = 0;
	unsigned char* data = NULL;
	int version = 0;
	if (XGetWindowProperty(dpy, win, awareAtom, 0, 1, False, AnyPropertyType,
	        &type, &fmt, &nitems, &after, &data) == Success) {
		if (data != NULL && nitems > 0 && fmt == 32) {
			version = (int)(((unsigned long*)data)[0] & 0xFF);
		}
		if (data != NULL) {
			XFree(data);
		}
	}
	return version;
}

// findTarget descends the window tree at root coordinates (rx, ry) to the
// deepest window under the pointer, then walks up the ancestor chain to the
// first XdndAware window. It returns that window (or None) and stores its
// protocol version in *outVer.
static Window findTarget(Display* dpy, Window root, int rx, int ry, Atom awareAtom, int* outVer) {
	*outVer = 0;

	// Descend: XTranslateCoordinates(root -> under) reports the child of `under`
	// containing (rx, ry). Iterate with the source fixed at root (coords stay
	// root-relative) until there is no deeper child.
	Window under = root, child = root;
	while (child != None) {
		under = child;
		int dx = 0, dy = 0;
		if (!XTranslateCoordinates(dpy, root, under, rx, ry, &dx, &dy, &child)) {
			break;
		}
		(void)dx;
		(void)dy;
	}

	// Walk up from the deepest window looking for XdndAware (a window's drop
	// target may be an ancestor of the exact leaf under the pointer).
	Window w = under;
	while (w != None && w != root) {
		int v = xdndAwareVersion(dpy, w, awareAtom);
		if (v > 0) {
			*outVer = v;
			return w;
		}
		Window r2 = None, parent = None, *children = NULL;
		unsigned int n = 0;
		if (!XQueryTree(dpy, w, &r2, &parent, &children, &n)) {
			break;
		}
		if (children != NULL) {
			XFree(children);
		}
		w = parent;
	}
	return None;
}

// sendXdnd sends an XDND ClientMessage (format 32) to window `to`; data.l[0] is
// always the source window per protocol convention, l1..l4 are message-specific.
static void sendXdnd(Display* dpy, Window to, Window src, Atom msg, long l1, long l2, long l3, long l4) {
	XEvent ev;
	memset(&ev, 0, sizeof(ev));
	ev.xclient.type = ClientMessage;
	ev.xclient.display = dpy;
	ev.xclient.window = to;
	ev.xclient.message_type = msg;
	ev.xclient.format = 32;
	ev.xclient.data.l[0] = (long)src;
	ev.xclient.data.l[1] = l1;
	ev.xclient.data.l[2] = l2;
	ev.xclient.data.l[3] = l3;
	ev.xclient.data.l[4] = l4;
	XSendEvent(dpy, to, False, NoEventMask, &ev);
	XFlush(dpy);
}

// serveSelection answers a target's XConvertSelection of the XdndSelection by
// writing the matching flavor (or the TARGETS list) onto the requestor window
// and replying with a SelectionNotify.
static void serveSelection(Display* dpy, XSelectionRequestEvent* req,
    BelluiDragFlavor* flavors, Atom* atoms, int count, Atom targetsAtom) {
	Atom prop = req->property;
	if (prop == None) {
		prop = req->target; // obsolete clients leave property unset
	}

	XEvent notify;
	memset(&notify, 0, sizeof(notify));
	notify.xselection.type = SelectionNotify;
	notify.xselection.display = req->display;
	notify.xselection.requestor = req->requestor;
	notify.xselection.selection = req->selection;
	notify.xselection.target = req->target;
	notify.xselection.time = req->time;
	notify.xselection.property = None; // set to prop on success

	if (req->target == targetsAtom) {
		// Report every supported target atom, plus TARGETS itself.
		Atom* list = (Atom*)malloc(sizeof(Atom) * (size_t)(count + 1));
		if (list != NULL) {
			for (int i = 0; i < count; i++) {
				list[i] = atoms[i];
			}
			list[count] = targetsAtom;
			XChangeProperty(dpy, req->requestor, prop, XA_ATOM, 32,
			    PropModeReplace, (unsigned char*)list, count + 1);
			free(list);
			notify.xselection.property = prop;
		}
	} else {
		for (int i = 0; i < count; i++) {
			if (req->target == atoms[i]) {
				XChangeProperty(dpy, req->requestor, prop, req->target, 8,
				    PropModeReplace, (const unsigned char*)flavors[i].data, flavors[i].len);
				notify.xselection.property = prop;
				break;
			}
		}
	}

	XSendEvent(dpy, req->requestor, False, NoEventMask, &notify);
	XFlush(dpy);
}

// getServerTime returns a valid server timestamp by triggering a PropertyNotify
// (a zero-length append) and reading its time. XDND positions/drops and the
// selection conversion want a real timestamp rather than CurrentTime.
static Time getServerTime(Display* dpy, Window win, Atom prop) {
	XChangeProperty(dpy, win, prop, XA_ATOM, 32, PropModeAppend, NULL, 0);
	for (;;) {
		XEvent ev;
		XWindowEvent(dpy, win, PropertyChangeMask, &ev);
		if (ev.type == PropertyNotify && ev.xproperty.atom == prop) {
			return ev.xproperty.time;
		}
	}
}

// waitForFinished serves SelectionRequests after a drop until the target sends
// XdndFinished (or a timeout). For a v5 target the accept bit is data.l[1]&1;
// pre-v5 targets carry no flag, so any XdndFinished (or a timeout after we
// dropped on a willing target) is treated as accepted.
static int waitForFinished(Display* dpy, XdndAtoms* A, int targetVer,
    BelluiDragFlavor* flavors, Atom* atoms, int count) {
	struct timespec start;
	clock_gettime(CLOCK_MONOTONIC, &start);
	// Return the moment XdndFinished arrives: per spec the target has finished
	// its conversions by then, and keeping the loop (and our XdndSelection
	// ownership) alive any longer overlaps the next gesture's drag loop — the
	// teardown then revoked the newer loop's selection ownership and its drop
	// could not convert (pre-rebrand tracker #933: the "stuck highlight" on the target).
	for (;;) {
		while (XPending(dpy) > 0) {
			XEvent ev;
			XNextEvent(dpy, &ev);
			if (ev.type == SelectionRequest) {
				serveSelection(dpy, &ev.xselectionrequest, flavors, atoms, count, A->targets);
			} else if (ev.type == ClientMessage && ev.xclient.message_type == A->finished) {
				if (targetVer < 5) {
					return 1;
				}
				return (ev.xclient.data.l[1] & 1) ? 1 : 0;
			}
		}
		struct timespec cur;
		clock_gettime(CLOCK_MONOTONIC, &cur);
		double elapsed = (double)(cur.tv_sec - start.tv_sec) +
		    (double)(cur.tv_nsec - start.tv_nsec) / 1e9;
		if (elapsed > 5.0) {
			return 1; // target accepted the drop but never confirmed; assume done
		}
		napSleep(10);
	}
}

// DragIcon holds the resources for the trailing drag-icon window so they can be
// torn down together when the drag ends.
typedef struct {
	Window   win;  // None when no icon (missing image or no ARGB visual)
	Pixmap   pix;
	Colormap cmap;
	int      w, h;
	int      mapped;
} DragIcon;

// ICON_OFFSET nudges the icon down-right of the pointer so the pointer hotspot
// never sits inside the icon window (belt-and-suspenders alongside the empty
// input shape below); the trailing offset also reads as "held by the cursor".
#define ICON_OFFSET 6

// createDragIcon builds a click-through, override-redirect ARGB window showing
// the iconW×iconH premultiplied-BGRA image, or leaves icon->win == None when no
// image is given or the server has no 32-bit TrueColor (ARGB) visual — the icon
// is purely cosmetic, so any failure degrades to the before pre-rebrand tracker #865 no-feedback drag.
static void createDragIcon(Display* dpy, int screen, Window root,
    const unsigned char* argb, int w, int h, DragIcon* icon) {
	memset(icon, 0, sizeof(*icon));
	if (argb == NULL || w <= 0 || h <= 0) {
		return;
	}

	// A 32-bit TrueColor visual gives per-pixel alpha under a compositor (the
	// common desktop case, e.g. GNOME/Mutter from the pre-rebrand tracker #865 report). Without one
	// we skip the icon rather than draw an opaque box with black fringes.
	XVisualInfo vinfo;
	if (!XMatchVisualInfo(dpy, screen, 32, TrueColor, &vinfo)) {
		return;
	}

	Colormap cmap = XCreateColormap(dpy, root, vinfo.visual, AllocNone);
	XSetWindowAttributes swa;
	memset(&swa, 0, sizeof(swa));
	swa.override_redirect = True; // not managed by the WM; no decorations/focus
	swa.background_pixel = 0;
	swa.border_pixel = 0;
	swa.colormap = cmap;
	swa.event_mask = ExposureMask;
	unsigned long wmask = CWOverrideRedirect | CWBackPixel | CWBorderPixel |
	    CWColormap | CWEventMask;
	Window win = XCreateWindow(dpy, root, -1000, -1000, (unsigned)w, (unsigned)h,
	    0, 32, InputOutput, vinfo.visual, wmask, &swa);
	if (win == None) {
		XFreeColormap(dpy, cmap);
		return;
	}

	// Declare the window a drag-and-drop icon (_NET_WM_WINDOW_TYPE_DND) so the
	// compositor/WM excludes it from XDND target tracking. Without this, a raised
	// follower window sits in the drag path and derails Mutter's XWayland→Wayland
	// drop routing — the drop never engages the real target and the source's grab
	// isn't released for ~30s (pre-rebrand tracker #933). The empty input shape below is not enough:
	// it hides the window from *input*, not from the compositor's DnD hit-test.
	// Must be set before the window is mapped.
	Atom netWmWindowType = XInternAtom(dpy, "_NET_WM_WINDOW_TYPE", False);
	Atom netWmWindowTypeDnd = XInternAtom(dpy, "_NET_WM_WINDOW_TYPE_DND", False);
	XChangeProperty(dpy, win, netWmWindowType, XA_ATOM, 32, PropModeReplace,
	    (unsigned char*)&netWmWindowTypeDnd, 1);

	// Make the window transparent to pointer input (empty input shape) so it is
	// invisible to XQueryPointer/XTranslateCoordinates target detection even if a
	// fast pointer move momentarily overtakes it between poll samples. The SHAPE
	// extension is near-universal; if absent, the +ICON_OFFSET keeps the pointer
	// clear anyway, so we simply skip the input shape.
	int shapeEventBase = 0, shapeErrorBase = 0;
	if (XShapeQueryExtension(dpy, &shapeEventBase, &shapeErrorBase)) {
		XShapeCombineRectangles(dpy, win, ShapeInput, 0, 0, NULL, 0, ShapeSet, Unsorted);
	}

	// Copy the premultiplied BGRA bytes into an XImage and blit them onto a
	// depth-32 pixmap used as the window background (the server repaints it on
	// every move/expose, so no expose handler is needed).
	size_t bytes = (size_t)w * (size_t)h * 4;
	char* buf = (char*)malloc(bytes);
	Pixmap pix = None;
	if (buf != NULL) {
		memcpy(buf, argb, bytes);
		XImage* img = XCreateImage(dpy, vinfo.visual, 32, ZPixmap, 0, buf,
		    (unsigned)w, (unsigned)h, 32, 0);
		if (img != NULL) {
			img->byte_order = LSBFirst; // caller sends B,G,R,A byte order
			pix = XCreatePixmap(dpy, win, (unsigned)w, (unsigned)h, 32);
			GC gc = XCreateGC(dpy, pix, 0, NULL);
			XPutImage(dpy, pix, gc, img, 0, 0, 0, 0, (unsigned)w, (unsigned)h);
			XFreeGC(dpy, gc);
			XSetWindowBackgroundPixmap(dpy, win, pix);
			XDestroyImage(img); // also frees buf
		} else {
			free(buf);
		}
	}

	icon->win = win;
	icon->pix = pix;
	icon->cmap = cmap;
	icon->w = w;
	icon->h = h;
	icon->mapped = 0;
}

// moveDragIcon repositions the icon at the pointer (offset), mapping it raised on
// first use so there is no flash at its off-screen spawn point.
static void moveDragIcon(Display* dpy, DragIcon* icon, int rx, int ry) {
	if (icon->win == None) {
		return;
	}
	XMoveWindow(dpy, icon->win, rx + ICON_OFFSET, ry + ICON_OFFSET);
	if (!icon->mapped) {
		XMapRaised(dpy, icon->win);
		icon->mapped = 1;
	}
}

// destroyDragIcon tears down every icon resource. It unmaps first and XSyncs so
// the server (and compositor) process the removal immediately — a destroy left
// queued behind the drop handshake keeps the ghost's pixels on screen until the
// next repaint (pre-rebrand tracker #919/#933).
static void destroyDragIcon(Display* dpy, DragIcon* icon) {
	if (icon->win == None) {
		return;
	}
	if (icon->mapped) {
		XUnmapWindow(dpy, icon->win);
		icon->mapped = 0;
	}
	XDestroyWindow(dpy, icon->win);
	if (icon->pix != None) {
		XFreePixmap(dpy, icon->pix);
	}
	if (icon->cmap != None) {
		XFreeColormap(dpy, icon->cmap);
	}
	XSync(dpy, False);
	memset(icon, 0, sizeof(*icon));
}

int belluiX11Available(void) {
	Display* dpy = XOpenDisplay(NULL);
	if (dpy == NULL) {
		return 0;
	}
	XCloseDisplay(dpy);
	return 1;
}

int belluiStartDragX11(BelluiDragFlavor* flavors, int count,
    const unsigned char* iconARGB, int iconW, int iconH) {
	if (flavors == NULL || count <= 0) {
		return 0;
	}

	// Private connection, used only by this (locked) thread — no XInitThreads
	// needed and no interference with GLFW's own Display.
	Display* dpy = XOpenDisplay(NULL);
	if (dpy == NULL) {
		return 0;
	}

	XdndAtoms A;
	internAtoms(dpy, &A);
	Atom timeProp = XInternAtom(dpy, "_BELLUI_XDND_TIME", False);

	Atom* atoms = (Atom*)malloc(sizeof(Atom) * (size_t)count);
	if (atoms == NULL) {
		XCloseDisplay(dpy);
		return 0;
	}
	for (int i = 0; i < count; i++) {
		atoms[i] = XInternAtom(dpy, flavors[i].mime, False);
	}

	int screen = DefaultScreen(dpy);
	Window root = RootWindow(dpy, screen);

	// Invisible InputOnly source window: owns the selection and receives the
	// target's XdndStatus/XdndFinished ClientMessages and our PropertyNotify.
	XSetWindowAttributes swa;
	memset(&swa, 0, sizeof(swa));
	swa.event_mask = PropertyChangeMask;
	Window src = XCreateWindow(dpy, root, -100, -100, 1, 1, 0, 0,
	    InputOnly, CopyFromParent, CWEventMask, &swa);

	XSetSelectionOwner(dpy, A.selection, src, CurrentTime);
	XFlush(dpy);
	if (XGetSelectionOwner(dpy, A.selection) != src) {
		XDestroyWindow(dpy, src);
		XCloseDisplay(dpy);
		free(atoms);
		return 0;
	}

	// Advertise the full type list; targets read it (XdndTypeList) when a source
	// offers more than three types.
	XChangeProperty(dpy, src, A.typelist, XA_ATOM, 32, PropModeReplace,
	    (unsigned char*)atoms, count);

	Time now = getServerTime(dpy, src, timeProp);

	// Cosmetic drag-icon window trailing the pointer (pre-rebrand tracker #865). Created on the same
	// private connection; the compositor redirects/blends it like any window.
	DragIcon icon;
	createDragIcon(dpy, screen, root, iconARGB, iconW, iconH, &icon);

	Window curTarget = None; // target we have XdndEnter'd
	int curVer = 0;          // its protocol version
	int accepted = 0;        // last XdndStatus accept flag from curTarget
	int result = 0;

	// Target-change debounce (pre-rebrand tracker #933): on XWayland the compositor bridges XDND to
	// Wayland through a 1x1 window it keeps moving under the cursor (Mutter's
	// dnd window). Polling XQueryPointer races those moves — a single missed
	// sample used to make us XdndLeave the bridge, which tears down the
	// Wayland-side drag for good and the drop then lands on whatever X window
	// sits geometrically beneath (often our own — a silent self-drop). Only
	// switch targets after the new result is stable for a few consecutive
	// samples, and require a longer streak for "no target at all" so a blip
	// never kills the bridge.
	Window pendTarget = None;
	int pendVer = 0;
	int pendCount = 0;

	for (;;) {
		// Drain events: serve conversions and track the target's accept state.
		while (XPending(dpy) > 0) {
			XEvent ev;
			XNextEvent(dpy, &ev);
			if (ev.type == SelectionRequest) {
				serveSelection(dpy, &ev.xselectionrequest, flavors, atoms, count, A.targets);
			} else if (ev.type == ClientMessage && ev.xclient.message_type == A.status) {
				// Per XDND, data.l[0] is the target window that sent the status —
				// but Mutter's XWayland bridge stamps its *current* proxy window,
				// which it rotates on every focus change, so the id may not match
				// the window we messaged (pre-rebrand tracker #933). Only our current negotiation ever
				// sends us XdndStatus, so accept it whenever a target is live.
				if (curTarget != None) {
					accepted = (ev.xclient.data.l[1] & 1) ? 1 : 0;
				}
			}
		}

		Window rr = None, cc = None;
		int rx = 0, ry = 0, wx = 0, wy = 0;
		unsigned int mask = 0;
		if (!XQueryPointer(dpy, root, &rr, &cc, &rx, &ry, &wx, &wy, &mask)) {
			break; // pointer left this screen — abandon the drag
		}
		(void)rr;
		(void)cc;
		(void)wx;
		(void)wy;

		// Trail the icon to the current pointer before target detection; its empty
		// input shape keeps it out of findTarget's window search.
		moveDragIcon(dpy, &icon, rx, ry);

		int ver = 0;
		Window tgt = findTarget(dpy, root, rx, ry, A.aware, &ver);

		if (tgt == curTarget) {
			pendCount = 0; // stable on the current target
		} else {
			if (tgt == pendTarget) {
				pendCount++;
			} else {
				pendTarget = tgt;
				pendVer = ver;
				pendCount = 1;
			}
			// A brief None must never detach us from a live target (see above);
			// a switch to a different XdndAware window confirms faster.
			int need = (tgt == None) ? 8 : 3;
			if (curTarget == None) {
				need = 1; // nothing to protect; enter new targets immediately
			}
			if (pendCount >= need) {
				if (curTarget != None) {
					sendXdnd(dpy, curTarget, src, A.leave, 0, 0, 0, 0);
				}
				curTarget = tgt;
				curVer = pendVer;
				accepted = 0;
				pendTarget = None;
				pendCount = 0;
				if (curTarget != None) {
					int v = curVer < BELLUI_XDND_VERSION ? curVer : BELLUI_XDND_VERSION;
					long l1 = ((long)v << 24) | (count > 3 ? 1 : 0);
					long t0 = count > 0 ? (long)atoms[0] : 0;
					long t1 = count > 1 ? (long)atoms[1] : 0;
					long t2 = count > 2 ? (long)atoms[2] : 0;
					sendXdnd(dpy, curTarget, src, A.enter, l1, t0, t1, t2);
				}
			}
		}

		if (curTarget != None) {
			sendXdnd(dpy, curTarget, src, A.position, 0,
			    ((long)rx << 16) | (ry & 0xFFFF), (long)now, (long)A.actionCopy);
		}

		// Button 1 is held for the whole drag; its release ends it.
		if ((mask & Button1Mask) == 0) {
			// The gesture is over the instant the button is released; remove the
			// trailing icon now so it never lingers through the accept-grace wait
			// and the XdndDrop/XdndFinished handshake below (pre-rebrand tracker #919).
			destroyDragIcon(dpy, &icon);

			// On a quick drag the target's XdndStatus (accept) may not have
			// arrived yet; give it a brief grace window before deciding, draining
			// selection requests meanwhile, so a fast drop is not lost.
			if (curTarget != None && !accepted) {
				struct timespec gs;
				clock_gettime(CLOCK_MONOTONIC, &gs);
				while (!accepted) {
					while (XPending(dpy) > 0) {
						XEvent ev;
						XNextEvent(dpy, &ev);
						if (ev.type == SelectionRequest) {
							serveSelection(dpy, &ev.xselectionrequest, flavors, atoms, count, A.targets);
						} else if (ev.type == ClientMessage && ev.xclient.message_type == A.status) {
							// See the drain loop above: Mutter stamps a rotated
							// proxy-window id, so don't match on data.l[0] (pre-rebrand tracker #933).
							accepted = (ev.xclient.data.l[1] & 1) ? 1 : 0;
						}
					}
					struct timespec gc;
					clock_gettime(CLOCK_MONOTONIC, &gc);
					double waited = (double)(gc.tv_sec - gs.tv_sec) +
					    (double)(gc.tv_nsec - gs.tv_nsec) / 1e9;
					if (accepted || waited > 0.3) {
						break;
					}
					napSleep(10);
				}
			}
			if (curTarget != None && accepted) {
				sendXdnd(dpy, curTarget, src, A.drop, 0, (long)now, 0, 0);
				result = waitForFinished(dpy, &A, curVer, flavors, atoms, count);
			} else if (curTarget != None) {
				sendXdnd(dpy, curTarget, src, A.leave, 0, 0, 0, 0);
			} else {
			}
			break;
		}

		napSleep(15);
	}

	destroyDragIcon(dpy, &icon);
	// Only clear the XdndSelection if this drag still owns it — SetSelectionOwner
	// is last-write-wins, so an unconditional clear here would revoke the
	// ownership of a NEWER drag loop that started while this one was finishing,
	// killing its data transfer (pre-rebrand tracker #933).
	if (XGetSelectionOwner(dpy, A.selection) == src) {
		XSetSelectionOwner(dpy, A.selection, None, CurrentTime);
	}
	XDestroyWindow(dpy, src);
	XCloseDisplay(dpy);
	free(atoms);
	return result;
}
