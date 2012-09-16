package main

import (
	"fmt"
	"image"

	"github.com/BurntSushi/xgb/xproto"

	"github.com/BurntSushi/xgbutil"
	"github.com/BurntSushi/xgbutil/ewmh"
	"github.com/BurntSushi/xgbutil/icccm"
	"github.com/BurntSushi/xgbutil/keybind"
	"github.com/BurntSushi/xgbutil/mousebind"
	"github.com/BurntSushi/xgbutil/xevent"
	"github.com/BurntSushi/xgbutil/xgraphics"
	"github.com/BurntSushi/xgbutil/xwindow"
)

// While the canvas and the window are essentialy the same, the canvas
// focuses on the abstraction of drawing some image into a viewport while the
// window focuses on the more X related aspects of setting up the canvas.
type window struct {
	*xwindow.Window
}

// newWindow creates the window, initializes the keybind and mousebind packages
// and sets up the window to act like a real top-level client.
func newWindow(X *xgbutil.XUtil) *window {
	xwin, err := xwindow.Generate(X)
	if err != nil {
		errLg.Fatalf("Could not create window: %s", err)
	}

	w := &window{xwin}

	keybind.Initialize(w.X)
	mousebind.Initialize(w.X)

	err = w.CreateChecked(w.X.RootWin(), 0, 0, 600, 600, xproto.CwBackPixel, 0xffffff)
	if err != nil {
		errLg.Fatalf("Could not create window: %s", err)
	}

	// Make the window close gracefully using the WM_DELETE_WINDOW protocol.
	w.WMGracefulClose(func(w *xwindow.Window) {
		xevent.Detach(w.X, w.Id)
		keybind.Detach(w.X, w.Id)
		mousebind.Detach(w.X, w.Id)
		w.Destroy()
		xevent.Quit(w.X)
	})

	// Set WM_STATE so it is interpreted as top-level and is mapped.
	err = icccm.WmStateSet(w.X, w.Id, &icccm.WmState{State: icccm.StateNormal})
	if err != nil {
		lg("Could not set WM_STATE: %s", err)
	}

	// _NET_WM_STATE = _NET_WM_STATE_NORMAL
	// not needed because we we set FS later anyway?
	//ewmh.WmStateSet(w.X, w.Id, []string{"_NET_WM_STATE_NORMAL"})

	w.nameSet("VImg")

	w.Map()

	err = ewmh.WmStateReq(w.X, w.Id, ewmh.StateToggle, "_NET_WM_STATE_FULLSCREEN")
	if err != nil {
		lg("Failed to go FullScreen:", err)
	}
	return w
}

// paint uses the xgbutil/xgraphics package to copy the area corresponding
// to ximg in its pixmap to the window. It will also issue a clear request
// before hand to try and avoid artifacts.
func (w *window) paint(ximg *xgraphics.Image) {
	dst := vpCenter(ximg, w.Geom.Width(), w.Geom.Height())
	ximg.XExpPaint(w.Id, dst.X, dst.Y)
}

// nameSet will set the name of the window and emit a benign message to
// verbose output if it fails.
func (w *window) nameSet(name string) {
	// Set _NET_WM_NAME so it looks nice.
	err := ewmh.WmNameSet(w.X, w.Id, fmt.Sprintf("vimg :: %s", name))
	if err != nil { // not a fatal error
		lg("Could not set _NET_WM_NAME: %s", err)
	}
}

// setupEventHandlers attaches the canvas' channels to the window and
// sets the appropriate callbacks to some events:
// ConfigureNotify events will cause the window to update its state of geometry.
// Expose events will cause the window to repaint the current image.
// Button events to allow panning.
// Key events to perform various tasks when certain keys are pressed.
func (w *window) setupEventHandlers(chans chans) {
	w.Listen(xproto.EventMaskStructureNotify | xproto.EventMaskExposure |
		xproto.EventMaskButtonPress | xproto.EventMaskButtonRelease | xproto.EventMaskKeyPress)

	// Get the current geometry in case we don't get a ConfigureNotify event
	// (or have already missed it).
	if _, err := w.Geometry(); err != nil {
		errLg.Fatal(err)
	}

	// Keep a state of window geometry.
	xevent.ConfigureNotifyFun(
		func(X *xgbutil.XUtil, ev xevent.ConfigureNotifyEvent) {
			w.Geom.WidthSet(int(ev.Width))
			w.Geom.HeightSet(int(ev.Height))
		}).Connect(w.X, w.Id)

	// Repaint the window on expose events.
	xevent.ExposeFun(
		func(X *xgbutil.XUtil, ev xevent.ExposeEvent) {
			chans.ctl <- []string{"pan", "origin"}
		}).Connect(w.X, w.Id)

	// Setup a drag handler to allow panning.
	mousebind.Drag(w.X, w.Id, w.Id, "1", false,
		func(X *xgbutil.XUtil, rx, ry, ex, ey int) (bool, xproto.Cursor) {
			chans.panStartChan <- image.Point{ex, ey}
			return true, 0
		},
		func(X *xgbutil.XUtil, rx, ry, ex, ey int) {
			chans.panStepChan <- image.Point{ex, ey}
		},
		// We do nothing on mouse release
		func(X *xgbutil.XUtil, rx, ry, ex, ey int) { return })

	for _, kb := range keybinds {
		k := kb // Needed because the callback closure will capture kb
		err := keybind.KeyPressFun(
			func(X *xgbutil.XUtil, ev xevent.KeyPressEvent) {
				chans.ctl <- k.command
			}).Connect(w.X, w.Id, k.key, false)
		if err != nil {
			errLg.Println(err)
		}
	}
}
