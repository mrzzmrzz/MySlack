//go:build !darwin

package main

// quickLook has no system preview panel to show files in outside macOS.
func quickLook(paths []string, index int) bool { return false }
