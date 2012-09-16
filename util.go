package main

import (
	"image"
	"io"
	"log"
	"os"
	"os/exec"

	"github.com/BurntSushi/xgbutil/xgraphics"
)

// vpCenter inspects the canvas and image geometry, and determines where the
// origin of the image should be painted into the canvas.
// If the image is bigger than the canvas, this is always (0, 0).
// If the image is the same size, then it is also (0, 0).
// If a dimension of the image is smaller than the canvas, then:
// x = (canvas_width - image_width) / 2 and
// y = (canvas_height - image_height) / 2
func vpCenter(ximg *xgraphics.Image, canWidth, canHeight int) image.Point {
	xmargin, ymargin := 0, 0
	if ximg.Bounds().Dx() < canWidth {
		xmargin = (canWidth - ximg.Bounds().Dx()) / 2
	}
	if ximg.Bounds().Dy() < canHeight {
		ymargin = (canHeight - ximg.Bounds().Dy()) / 2
	}
	return image.Point{xmargin, ymargin}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// Logging
var errLg = log.New(os.Stderr, "[vimg error] ", log.Lshortfile)

// lg is a convenient alias for printing verbose output.
func lg(format string, v ...interface{}) {
	if flagVerbose {
		log.Printf(format, v...)
	}
}

//

func runExternal(cmds []string, img string) {
	for i := range cmds {
		if cmds[i] == "%" {
			cmds[i] = img
		}
	}
	errLg.Println(cmds)
	c := exec.Command(cmds[0], cmds[1:]...)
	er, _ := c.StderrPipe()
	ou, _ := c.StdoutPipe()
	c.Start()
	go func() { io.Copy(os.Stdout, ou) }()
	go func() { io.Copy(os.Stderr, er) }()
}
