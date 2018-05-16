package main

import (
	"github.com/v2pro/plz/witch"
	"os"
)

func main() {
	addr := os.Getenv("WITCH_VIEWER")
	if addr == "" {
		addr = ":8318"
	}
	witch.StartViewer(addr)
}
