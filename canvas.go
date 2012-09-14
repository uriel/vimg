package main

import (
	"fmt"
	"image"
	"runtime"

	"github.com/BurntSushi/xgbutil/xevent"
)

type chans struct {
	ctl chan cmd

	// The pan{Start,Step}Chan channels for "drag start", "drag step".
	panStartChan chan image.Point
	panStepChan  chan image.Point
}

func load(win *window, im *Img) {
	if im.vimage != nil {
		return
	}
	// Skip image if already loaded.
	select {
	case vi := <-im.load:
		im.load <- vi
		return
	default:
	}

	v := newImage(win, im)

	// Tell the canvas that this image has been loaded.
	select {
	case im.load <- v:
	default:
		lg("XXXX Somebody else loaded img faster than us! %v", im)
	}
}

func loader(win *window, imgs chan *Img) {
	for im := range imgs {
		load(win, im)
		runtime.Gosched()
	}
}

var preloaders chan *Img

const PreloadQueueSize = 32

func preload(win *window, imgs []Img, idx int) {

	if preloaders == nil {
		preloaders = make(chan *Img, PreloadQueueSize)
		for i := 0; i < runtime.NumCPU(); i++ {
			go loader(win, preloaders)
		}
	}

	// TODO: Should wrap arround!
	// TODO: Replace FIFO with "priority queue" based on distance from idx
	for i := 0; i <= PreloadQueueSize && i+idx < len(imgs); i++ {
		img := &imgs[i+idx]
		if img.vimage == nil && img.loading == false {
			select {
			case preloaders <- img:
				img.loading = true
			default:
				// Preload queue full
				break
			}
		}
	}
	// TODO: 'Garbage collect' far away images when memory starts to become low.
	//fidx := idx+len(imgs)/2 // Start freeing as far away as possible
	// TODO: preload also for when iterating backwards? could use last step to predict
	// the general iteraction direction.
}

// canvas is meant to be run as a single goroutine that maintains the state
// of the image viewer. It manipulates state by reading values from the channels
// defined in the 'chans' type.
func canvas(win *window, imgs []Img) {
	chans := chans{
		ctl: make(chan cmd, 0),

		panStartChan: make(chan image.Point, 0),
		panStepChan:  make(chan image.Point, 0),
	}

	win.setupEventHandlers(chans)
	current := 0
	origin := image.Point{0, 0}

	setImage := func(i int, pt image.Point) {
		if i >= len(imgs) {
			i = 0
		}
		if i < 0 {
			i = len(imgs) - 1
		}
		if current != i {
			win.ClearAll()
		}

		current = i

		img := &imgs[i]
		//lg("setImage %d, %v, %s", i, img.vimage, img.name)
		if img.vimage == nil {
			win.nameSet(fmt.Sprintf("%s - Loading... %d", img.name, i))
			// TODO Maybe we should check if a preloader is already working on this image.
			img.loading = true
			load(win, img)
			img.vimage = <-img.load
		}

		if img.vimage.err != nil {
			win.nameSet(fmt.Sprintf("%s - Error loading... %s", img.name, img.vimage.err))
			// Should we call preload() anyway?
			return
		}

		lg("setImage show() %d, %v, %d", i, img.vimage, len(img.load))
		origin = originTrans(pt, win, img.vimage)
		show(win, img, origin)
		preload(win, imgs, i+1)
	}

	go func() {
		panStart, panOrigin := image.Point{}, image.Point{}
		for {
			select {
			case cmd := <-chans.ctl:
				switch cmd[0] {
				case "next":
					setImage(current+1, image.Point{0, 0})

				case "prev":
					setImage(current-1, image.Point{0, 0})

				// resize the window to fit the current image exactly.
				case "fit":
					b := imgs[current].vimage.Bounds()
					win.Resize(b.Dx(), b.Dy())
				case "pan":
					p := image.Point{}
					switch cmd[1] {
					case "left":
						p = image.Point{origin.X - flagStepIncrement, origin.Y}
					case "right":
						p = image.Point{origin.X + flagStepIncrement, origin.Y}

					// up and down are reversed, X origin is the top-left corner
					case "up":
						p = image.Point{origin.X, origin.Y - flagStepIncrement}
					case "down":
						p = image.Point{origin.X, origin.Y + flagStepIncrement}
					case "origin":
						p = origin
					}
					setImage(current, p)
				case "quit":
					lg("Quit!")
					xevent.Quit(win.X)
				case "!":
					runExternal(cmd.Args(), imgs[current].name)
				default:
					errLg.Printf("Unrecognized command: %v", cmd)
				}
			case pt := <-chans.panStartChan:
				panStart = pt
				panOrigin = origin
			case pt := <-chans.panStepChan:
				xd, yd := panStart.X-pt.X, panStart.Y-pt.Y
				setImage(current, image.Point{xd + panOrigin.X, yd + panOrigin.Y})
			}
		}
	}()

	// Draw first image. 
	// If we always go FS we might not need this as we will get an X expose event.
	chans.ctl <- cmd{"pan", "NOWHERE"}
}

// originTrans translates the origin with respect to the current image and the
// current canvas size. This makes sure we never incorrectly position the image.
// (i.e., panning never goes too far, and whenever the canvas is bigger than
// the image, the origin is *always* (0, 0).
func originTrans(pt image.Point, win *window, img *vimage) image.Point {
	// If there's no valid image, then always return (0, 0).
	if img == nil {
		return image.Point{0, 0}
	}

	// Quick aliases.
	ww, wh := win.Geom.Width(), win.Geom.Height()
	dw := img.Bounds().Dx() - ww
	dh := img.Bounds().Dy() - wh

	// Set the allowable range of the origin point of the image.
	// i.e., never less than (0, 0) and never greater than the width/height
	// of the image that isn't viewable at any given point (which is determined
	// by the canvas size).
	pt.X = min(img.Bounds().Min.X+dw, max(pt.X, 0))
	pt.Y = min(img.Bounds().Min.Y+dh, max(pt.Y, 0))

	// Validate origin point. If the width/height of an image is smaller than
	// the canvas width/height, then the image origin cannot change in x/y
	// direction.
	if img.Bounds().Dx() < ww {
		pt.X = 0
	}
	if img.Bounds().Dy() < wh {
		pt.Y = 0
	}

	return pt
}

// show translates the given origin point, paints the appropriate part of the
// current image to the canvas, and sets the name of the window.
// (Painting only paints the sub-image that is viewable.)
func show(win *window, img *Img, pt image.Point) {
	// If there's no valid image, don't bother trying to show it.
	// (We're hopefully loading the image now.)
	if img.vimage == nil {
		panic("Should not happen!")
	}

	// Translate the origin to reflect the size of the image and canvas.
	pt = originTrans(pt, win, img.vimage)

	// Now paint the sub-image to the window.
	win.paint(img.vimage.SubImage(image.Rect(pt.X, pt.Y,
		pt.X+win.Geom.Width(), pt.Y+win.Geom.Height())))

	// Always set the name of the window when we update it with a new image.
	win.nameSet(fmt.Sprintf("%s (%dx%d)",
		img.name, img.vimage.Bounds().Dx(), img.vimage.Bounds().Dy()))
}
