package main

import (
	//"io"
	"log"
	"os"
	"os/exec"
)

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
	out, _ := c.CombinedOutput()
	os.Stderr.Write(out)
	// Run command in background
	//er, _ := c.StderrPipe()
	//ou, _ := c.StdoutPipe()
	//c.Start()
	//go func() { io.Copy(os.Stdout, ou) }()
	//go func() { io.Copy(os.Stderr, er) }()
}
