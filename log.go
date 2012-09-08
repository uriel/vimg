package main

import (
	"log"
	"os"
)

var errLg = log.New(os.Stderr, "[vimg error] ", log.Lshortfile)

// lg is a convenient alias for printing verbose output.
func lg(format string, v ...interface{}) {
	if !flagVerbose {
		return
	}
	log.Printf(format, v...)
}
