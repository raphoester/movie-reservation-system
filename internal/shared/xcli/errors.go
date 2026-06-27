package xcli

import "errors"

// ErrInterrupted is returned by ReadMasked when the user presses Ctrl-C.
var ErrInterrupted = errors.New("interrupted")
