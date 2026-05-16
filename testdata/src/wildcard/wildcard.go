package wildcard

import "syscall"

func F() {
	_ = syscall.Getuid() // want `forbidden reference to syscall\.Getuid`
	_ = syscall.Stdin    // want `forbidden reference to syscall\.Stdin`
}
