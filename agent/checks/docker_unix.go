//go:build !windows
// +build !windows

package checks

const DefaultDockerHost = "unix:///var/run/docker.sock"
