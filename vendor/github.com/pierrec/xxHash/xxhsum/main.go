// Command line interface to the xxHash32 and xxHash64 packages.
// Usage:
// 	xxHash [-mode 0] [-seed 123] filename1 [filename2...]
// where
//  mode: hash mode (0=32bits, 1=64bits) (default=1)
//  seed: seed to be used (default=0)
package main

import (
	"flag"
	"fmt"
	"hash"
	"io"
	"os"

	"github.com/pierrec/xxHash/xxHash32"
	"github.com/pierrec/xxHash/xxHash64"
)

func main() {
	seed := flag.Uint64("seed", 0, "seed value")
	mode := flag.Int("mode", 1, "hash mode: 0=32bits, 1=64bits")
	flag.Parse()

	var xxh hash.Hash
	if *mode == 0 {
		xxh = xxHash32.New(uint32(*seed))
	} else {
		xxh = xxHash64.New(*seed)
	}

	// Process each file in sequence
	for _, filename := range flag.Args() {
		inputFile, err := os.Open(filename)
		if err != nil {
			continue
		}
		if _, err := io.Copy(xxh, inputFile); err == nil {
			fmt.Printf("%x %s\n", xxh.Sum(nil), filename)
		}
		inputFile.Close()
		xxh.Reset()
	}
}
