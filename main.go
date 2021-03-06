package main

import (
	"flag"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"

	"github.com/BurntSushi/xgbutil"
	"github.com/BurntSushi/xgbutil/xevent"
)

var (
	flagVerbose bool
	flagProfile string

	window *Window
)

func init() {
	runtime.GOMAXPROCS(runtime.NumCPU())

	flag.BoolVar(&flagVerbose, "v", false, "Print logging output to stderr.")
	flag.StringVar(&flagProfile, "profile", "", "Save CPU profile to the given file.")
	flag.Usage = usage
	flag.Parse()
}

func usage() {
	fmt.Fprintf(os.Stderr, "Usage: vimg [flags] image-file [image-file ...]\n")
	flag.PrintDefaults()

	fmt.Print("\nControls:\n\n")
	for _, keyb := range keybinds {
		fmt.Printf("%-10s %s\n", keyb.key, keyb.desc)
	}
	fmt.Printf("%-10s %s\n", "mouse", "Left mouse button will pan the image.\n")

	os.Exit(2)
}

func main() {

	if flagProfile != "" {
		f, err := os.Create(flagProfile)
		if err != nil {
			errLg.Fatal(err)
		}
		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
	}

	if flag.NArg() == 0 {
		errLg.Print("No images specified.\n\n")
		usage()
	}

	// Connect to X and quit if we fail.
	X, err := xgbutil.NewConn()
	if err != nil {
		errLg.Fatal(err)
	}

	files := findFiles(flag.Args())

	if len(files) == 0 {
		errLg.Fatal("No images specified could be shown.")
	}

	canvas := Canvas{imgs: make([]*Img, 0, len(files))}
	for _, name := range files {
		canvas.imgs = append(canvas.imgs, &Img{name, make(chan *vimage, 1), false, nil})
	}

	chans := chans{
		ctl: make(chan cmd, 0),

		panStartChan: make(chan image.Point, 0),
		panStepChan:  make(chan image.Point, 0),
	}

	// Create the X window before starting anything so that the user knows
	// something is going on.
	window = newWindow(X)
	window.setName("VImg")
	window.setupEventHandlers(chans)

	// Create the canvas, this is the heart of the app
	go canvas.run(chans)

	// Start the main X event loop.
	xevent.Main(X)
}

func findFiles(args []string) (files []string) {
	for _, f := range args {
		fi, err := os.Stat(f)
		if err != nil {
			errLg.Print("Can't access", f, err)
		} else if fi.IsDir() {
			files = append(files, dirImages(f)...)
		} else {
			files = append(files, f)
		}
	}
	return
}

func dirImages(dir string) (files []string) {
	fs, _ := ioutil.ReadDir(dir)
	for _, f := range fs {
		if f.IsDir() {
			// For now ignore directories
			// Maybe we should add a recursive flag?

			// TODO filter by regexp
		} else if filepath.Ext(f.Name()) != "" {
			files = append(files, filepath.Join(dir, f.Name()))
		}
	}
	return
}
