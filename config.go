package main

type cmd []string

func (c cmd) Args() (args []string) {
	c = c[1:]
	for i := range c {
		args = append(args, c[i])
	}
	return
}

// keyb represents a keybinding.
type keyb struct {
	key     string // key sequence
	command cmd    // commands to 'run'
	desc    string // description
}

// A list of keybindings. Each value corresponds to a triple of the key
// sequence to bind to, the action to run when that key sequence is
// pressed and a quick description of what the keybinding does.
var keybinds = []keyb{
	{"left", cmd{"prev"}, "Cycle to the previous image."},
	{"right", cmd{"next"}, "Cycle to the next image."},

	{"shift-h", cmd{"prev"}, "Cycle to the previous image."},
	{"shift-l", cmd{"next"}, "Cycle to the next image."},

	{"r", cmd{"fit"}, "Resize the window to fit the current image."},
	{"shift-r", cmd{"!", "mv", "%", ".trash/"}, "Move file to .trash/."},

	{"h", cmd{"pan", "left"}, "Pan left."},
	{"j", cmd{"pan", "down"}, "Pan down."},
	{"k", cmd{"pan", "up"}, "Pan up."},
	{"l", cmd{"pan", "right"}, "Pan right."},
	{"q", cmd{"quit"}, "Quit."},
}
