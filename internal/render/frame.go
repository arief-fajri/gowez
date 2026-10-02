package render

// Frame is the complete command list for one displayed frame.
type Frame struct {
	// Width and Height are the frame dimensions in logical pixels.
	Width, Height int
	// Commands execute in order.
	Commands []Command
}
