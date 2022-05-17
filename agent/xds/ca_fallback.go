//go:build !(linux || dragonfly || freebsd || netbsd || openbsd || darwin || solaris)

package xds

// Possible certificate files; stop after finding one.
var certFiles = []string{}
