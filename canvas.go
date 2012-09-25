package main

import (
	"fmt"
	"image"
	"os"
	"runtime"
)

type chans struct {
	ctl chan cmd

	// The pan{Start,Step}Chan channels for "drag start", "drag step".
	panStartChan chan image.Point
	panStepChan  chan image.Point
}

func load(im *Img) (vimg *vimage) {
	if im.vimage != nil {
		return im.vimage
	}
	// Skip image if already loaded.
	select {
	case vimg = <-im.load:
		im.load <- vimg
		return
	default:
	}

	vimg = newImage(im)

	// Tell the canvas that this image has been loaded.
	select {
	case im.load <- vimg:
	default:
		lg("XXXX Somebody else loaded img faster than us! %v", im)
	}

	return
}

func loader(imgs chan *Img) {
	for im := range imgs {
		load(im)
		runtime.Gosched()
	}
}

var preloaders chan *Img

const PreloadQueueSize = 32

func preload(imgs []*Img, idx int) {

	if preloaders == nil {
		preloaders = make(chan *Img, PreloadQueueSize)
		for i := 0; i < runtime.NumCPU(); i++ {
			go loader(preloaders)
		}
	}

	// TODO: Should wrap arround!
	// TODO: Replace FIFO with "priority queue" based on distance from idx
	for i := 0; i <= PreloadQueueSize && i+idx < len(imgs); i++ {
		img := imgs[i+idx]
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
	// TODO: 'Garbage collect' far away images when free memory is low.
	//fidx := idx+len(imgs)/2 // Start freeing as far away as possible
	// TODO: preload also for when iterating backwards?
	// could use last step to predict the general iteraction direction.
}

type Canvas struct {
	imgs    []*Img
	i       *Img
	current int
	origin  image.Point
}

func (c *Canvas) delImage(i int) {
	c.imgs = append(c.imgs[:i], c.imgs[i+1:]...)
	c.setImage(c.current)
}

func (c *Canvas) setImage(i int) {
	if i >= len(c.imgs) {
		i = 0
	}
	if i < 0 {
		i = len(c.imgs) - 1
	}

	window.ClearAll()

	c.current = i
	c.i = c.imgs[i]
	if c.i.vimage == nil {
		window.setName(fmt.Sprintf("%s - Loading... ", c.i.name))
		// TODO Maybe should check if a preloader is already working on this image.
		c.i.loading = true
		c.i.vimage = load(c.i)
	}

	if c.i.vimage.err != nil {
		errLg.Printf("%s - Error loading... %s", c.i.name, c.i.vimage.err)
		c.delImage(i)
		return
	}

	c.origin = show(c.i, image.Point{0, 0})
	lg("show() %v, %d, %s", c.i.vimage, len(c.i.load), c.i.name)
	preload(c.imgs, i+1)
}

// canvas is meant to be run as goroutine that maintains the state of the image
// viewer. It manipulates state by reading values from the channels defined in
// the 'chans' type.
func (c *Canvas) run(chans chans) {

	c.setImage(c.current)

	panStart, panOrigin := image.Point{}, image.Point{}
	for {
		select {
		case cmd := <-chans.ctl:
			switch cmd[0] {
			case "next":
				c.setImage(c.current + 1)

			case "prev":
				c.setImage(c.current - 1)

			// resize the window to fit the current image.
			// Not needed since we are always full screen
			// and if we arent fs, resize maybe should be automatic
			//case "fit":
			//	b := imgs[current].vimage.Bounds()
			//	window.Resize(b.Dx(), b.Dy())
			case "pan":
				switch cmd[1] {
				case "left":
					c.origin.X -= panIncrement
				case "right":
					c.origin.X += panIncrement

				// up and down are reversed, X origin is the top-left corner
				case "up":
					c.origin.Y -= panIncrement
				case "down":
					c.origin.Y += panIncrement
				}
				c.origin = show(c.i, c.origin)
			case "quit":
				// Xgb bug prevents this from working?
				// Anything wrong with calling os.Exit() directly? 
				//xevent.Quit(window.X) 
				os.Exit(0)
			case "!":
				runExternal(cmd.Args(), c.i.name)
				if _, err := os.Stat(c.i.name); err != nil {
					c.delImage(c.current)
				}
			default:
				errLg.Printf("Unrecognized command: %v", cmd)
			}
		case pt := <-chans.panStartChan:
			panStart = pt
			panOrigin = c.origin
		case pt := <-chans.panStepChan:
			c.origin = show(c.i, panStart.Sub(pt).Add(panOrigin))
		}
	}
}

// originTrans translates the origin with respect to the current image and the
// current canvas size. This makes sure we never incorrectly position the image.
// (i.e., panning never goes too far, and whenever the canvas is bigger than
// the image, the origin is *always* (0, 0).
func originTrans(pt image.Point, win *Window, img *vimage) image.Point {
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

func show(img *Img, pt image.Point) image.Point {

	// Translate the origin to reflect the size of the image and canvas.
	pt = originTrans(pt, window, img.vimage)

	// Painting only paints the sub-image that is viewable.
	window.paint(img.vimage.SubImage(
		image.Rect(pt.X, pt.Y, pt.X+window.Geom.Width(), pt.Y+window.Geom.Height())))

	// Always set the name of the window when we update it with a new image.
	window.setName(img.name)

	return pt
}
