//go:build !windows

package main

import "os"

// enableColors reports whether the file is a terminal (a character
// device). Every terminal on Unix accepts escape sequences. /dev/null is a
// character device too, but nobody reads the colors there.
func enableColors(file *os.File) bool {
	info, err := file.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}
