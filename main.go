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

	"github.com/BurntSushi/xgbutil"
	"github.com/BurntSushi/xgbutil/xevent"
)

var (
	// When flagVerbose is true, logging output will be written to stderr.
	flagVerbose bool

	// Whether to run a CPU profile.
	flagProfile string
)

func init() {
	runtime.GOMAXPROCS(runtime.NumCPU())

	log.SetPrefix("[VImg] ")

	flag.BoolVar(&flagVerbose, "v", false, "Print logging output to stderr.")
	flag.StringVar(&flagProfile, "profile", "", "Save CPU profile to the file name provided.")
	flag.Usage = usage
	flag.Parse()
}

func usage() {
	fmt.Fprintf(os.Stderr, "Usage: vimg [flags] image-file [image-file ...]\n")
	flag.PrintDefaults()

	for _, keyb := range keybinds {
		fmt.Printf("%-10s %s\n", keyb.key, keyb.desc)
	}
	fmt.Printf("%-10s %s\n", "mouse", "Left mouse button will pan the image.")

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
	canvas(window, imgs)

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
	fs, _ := fd.Readdir(0)
	for _, f := range fs {
		if f.IsDir() {
			// TODO Maybe add a recursive flag?
			lg("Not loading subdir: ", dir, f.Name())

			// TODO filter by regexp
		} else if filepath.Ext(f.Name()) != "" {
			files = append(files, filepath.Join(dir, f.Name()))
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
