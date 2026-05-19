package a

import (
	"os"
	"syscall"
)

// In package `a`: os.Getenv is EXCLUDED for this package by rule
// "no-os-in-b", so no diagnostic here. syscall.* is forbidden by
// rule "no-syscall-in-a" with no exclusion → diagnostic fires.
func F() {
	_ = os.Getenv("HOME")
	_ = syscall.Getpid() // want `forbidden reference to syscall\.Getpid \(rule: no-syscall-in-a, pattern: syscall\.\*\)`
}
