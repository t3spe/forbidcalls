package b

import (
	"os"
	"syscall"
)

// In package `b`: os.Getenv is NOT excluded → rule "no-os-in-b" fires.
// syscall.* is EXCLUDED for this package by rule "no-syscall-in-a" → no
// diagnostic for syscall.Getpid.
func F() {
	_ = os.Getenv("HOME") // want `forbidden reference to os\.Getenv \(rule: no-os-in-b, pattern: os\.Getenv\)`
	_ = syscall.Getpid()
}
