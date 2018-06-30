// Command line utility for the lz4 package.
package main

import (
	// 	"bytes"

	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path"
	"runtime"
	"strings"

	"github.com/pierrec/lz4"
	"github.com/pkg/profile"
)

func main() {
	// Process command line arguments
	var (
		blockMaxSizeDefault  = 4 << 20
		flagStdout           = flag.Bool("c", false, "output to stdout")
		flagDecompress       = flag.Bool("d", false, "decompress flag")
		flagBlockMaxSize     = flag.Int("B", blockMaxSizeDefault, "block max size [64Kb,256Kb,1Mb,4Mb]")
		flagBlockChecksum    = flag.Bool("BX", false, "enable block checksum")
		flagStreamChecksum   = flag.Bool("Sx", false, "disable stream checksum")
		flagCompressionLevel = flag.Int("l", 0, "compression level (0=fastest)")
		profileName          = flag.String("p", "", "path to the profile file")
		mode                 = flag.String("profile.mode", "", "enable profiling mode, one of [cpu, mem, mutex, block]")
	)
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage:\n\t%s [arg] [input]...\n\tNo input means [de]compress stdin to stdout\n\n", os.Args[0])
		flag.PrintDefaults()
	}
	flag.Parse()

	ppath := profile.ProfilePath(*profileName)
	switch *mode {
	case "cpu":
		defer profile.Start(profile.CPUProfile, ppath).Stop()
	case "mem":
		defer profile.Start(profile.MemProfile, ppath).Stop()
	case "mutex":
		defer profile.Start(profile.MutexProfile, ppath).Stop()
	case "block":
		defer profile.Start(profile.BlockProfile, ppath).Stop()
	default:
		// do nothing
	}
	flag.Parse()

	// Use all CPUs
	runtime.GOMAXPROCS(runtime.NumCPU())

	zr := lz4.NewReader(nil)
	zw := lz4.NewWriter(nil)
	zh := lz4.Header{
		BlockChecksum:    *flagBlockChecksum,
		BlockMaxSize:     *flagBlockMaxSize,
		NoChecksum:       *flagStreamChecksum,
		CompressionLevel: *flagCompressionLevel,
	}

	worker := func(in io.Reader, out io.Writer) {
		if *flagDecompress {
			zr.Reset(in)
			if _, err := io.Copy(out, zr); err != nil {
				log.Fatalf("Error while decompressing input: %v", err)
			}
		} else {
			zw.Reset(out)
			zw.Header = zh
			if _, err := io.Copy(zw, in); err != nil {
				log.Fatalf("Error while compressing input: %v", err)
			}
			if err := zw.Close(); err != nil {
				log.Fatalf("Error while closing stream: %v", err)
			}
		}
	}

	// No input means [de]compress stdin to stdout
	if len(flag.Args()) == 0 {
		worker(os.Stdin, os.Stdout)
		os.Exit(0)
	}

	// Compress or decompress all input files
	for _, inputFileName := range flag.Args() {
		outputFileName := path.Clean(inputFileName)

		if !*flagStdout {
			if *flagDecompress {
				outputFileName = strings.TrimSuffix(outputFileName, lz4.Extension)
				if outputFileName == inputFileName {
					log.Fatalf("Invalid output file name: same as input: %s", inputFileName)
				}
			} else {
				outputFileName += lz4.Extension
			}
		}

		inputFile, err := os.Open(inputFileName)
		if err != nil {
			log.Fatalf("Error while opening input: %v", err)
		}

		outputFile := os.Stdout
		if !*flagStdout {
			outputFile, err = os.Create(outputFileName)
			if err != nil {
				log.Fatalf("Error while opening output: %v", err)
			}
		}
		worker(inputFile, outputFile)

		inputFile.Close()
		if !*flagStdout {
			outputFile.Close()
		}
	}
}
