package main

import (
	"flag"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"time"

	"github.com/BurntSushi/xgbutil"
	"github.com/BurntSushi/xgbutil/xevent"
)

var (
	// When flagVerbose is true, logging output will be written to stderr.
	flagVerbose bool

	// The initial width and height of the window.
	flagWidth, flagHeight int

	// The amount to increment panning when using h,j,k,l
	flagStepIncrement int

	// Whether to run a CPU profile.
	flagProfile string

	// Print all keybindings and exit.
	flagKeybindings bool

	// A list of keybindings. Each value corresponds to a triple of the key
	// sequence to bind to, the action to run when that key sequence is
	// pressed and a quick description of what the keybinding does.
	keybinds = []keyb{
		{"left", "Cycle to the previous image.", []string{"prev"}},
		{"right", "Cycle to the next image.", []string{"next"}},
		{"shift-h", "Cycle to the previous image.", []string{"prev"}},
		{"shift-l", "Cycle to the next image.", []string{"next"}},
		{"r", "Resize the window to fit the current image.", []string{"fit"}},
		{"h", "Pan left.", []string{"pan", "left"}},
		{"j", "Pan down.", []string{"pan", "down"}},
		{"k", "Pan up.", []string{"pan", "up"}},
		{"l", "Pan right.", []string{"pan", "right"}},
		{"q", "Quit.", []string{"quit"}},
	}
)

func init() {
	runtime.GOMAXPROCS(runtime.NumCPU())

	log.SetPrefix("[VImg] ")

	flag.BoolVar(&flagVerbose, "v", false, "Print logging output to stderr.")
	flag.IntVar(&flagWidth, "width", 600, "Initial window width.")
	flag.IntVar(&flagHeight, "height", 600, "Initial window.")
	flag.IntVar(&flagStepIncrement, "increment", 20, "Increment (in pixels) used to pan the image.")
	flag.StringVar(&flagProfile, "profile", "", "Save CPU profile to the file name provided.")
	flag.BoolVar(&flagKeybindings, "keybindings", false, "Output a list all keybindings.")
	flag.Usage = usage
	flag.Parse()

	if flagWidth == 0 || flagHeight == 0 {
		errLg.Fatal("The width and height must be non-zero values.")
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, "Usage: vimg [flags] image-file [image-file ...]\n")
	flag.PrintDefaults()
	os.Exit(2)
}

func main() {
	if flagKeybindings {
		for _, keyb := range keybinds {
			fmt.Printf("%-10s %s\n", keyb.key, keyb.desc)
		}
		fmt.Printf("%-10s %s\n", "mouse", "Left mouse button will pan the image.")
		os.Exit(0)
	}

	if flagProfile != "" {
		f, err := os.Create(flagProfile)
		if err != nil {
			errLg.Fatal(err)
		}
		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
	}

	if flag.NArg() == 0 {
		fmt.Fprint(os.Stderr, "\n")
		errLg.Print("No images specified.\n\n")
		usage()
	}

	// Connect to X and quit if we fail.
	X, err := xgbutil.NewConn()
	if err != nil {
		errLg.Fatal(err)
	}

	// Create the X window before starting anything so that the user knows
	// something is going on.
	window := newWindow(X)

	files := findFiles(flag.Args())

	if len(files) == 0 {
		errLg.Fatal("No images specified could be shown.")
	}

	imgs := make([]Img, 0, len(files))
	for _, name := range files {
		imgs = append(imgs, Img{nil, name, make(chan *vimage, 1), false, nil})
	}

	// Create the canvas, this is the heart of the app
	canvas(X, window, imgs)

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
	fd, _ := os.Open(dir)
	fs, _ := fd.Readdirnames(0)
	for _, f := range fs {
		// TODO filter by regexp
		if filepath.Ext(f) != "" {
			files = append(files, filepath.Join(dir, f))
		}
	}
	return
}

type Img struct {
	image   image.Image
	name    string
	load    chan *vimage
	loading bool // TODO: Maybe we should use a nil load chan instead
	vimage  *vimage
}

func decodeFile(name string) (img image.Image, err error) {
	file, err := os.Open(name)
	if err != nil {
		errLg.Println(err)
		return
	}

	start := time.Now()
	img, kind, err := image.Decode(file)
	if err != nil {
		errLg.Printf("Could not decode '%s' into a supported image "+"format: %s", name, err)
		return
	}
	lg("Decoded '%s' into image type '%s' (%s).", name, kind, time.Since(start))
	return
}
