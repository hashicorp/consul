//+build go1.9

package lz4_test

import (
	"fmt"
	"io/ioutil"
	"reflect"
	"testing"

	"github.com/pierrec/lz4"
)

type testcase struct {
	file         string
	compressible bool
	src          []byte
}

var rawFiles = []testcase{
	// {"testdata/207326ba-36f8-11e7-954a-aca46ba8ca73.png", true, nil},
	{"testdata/e.txt", true, nil},
	{"testdata/gettysburg.txt", true, nil},
	{"testdata/Mark.Twain-Tom.Sawyer.txt", true, nil},
	{"testdata/pg1661.txt", true, nil},
	{"testdata/pi.txt", true, nil},
	{"testdata/random.data", false, nil},
	{"testdata/repeat.txt", true, nil},
}

func TestCompressUncompressBlock(t *testing.T) {
	type compressor func(s, d []byte) (int, error)

	run := func(tc testcase, compress compressor) int {
		t.Helper()
		src := tc.src

		// Compress the data.
		zbuf := make([]byte, lz4.CompressBlockBound(len(src)))
		n, err := compress(src, zbuf)
		if err != nil {
			t.Error(err)
			return 0
		}
		zbuf = zbuf[:n]

		// Make sure that it was actually compressed unless not compressible.
		if !tc.compressible {
			return 0
		}

		if n == 0 || n >= len(src) {
			t.Errorf("data not compressed: %d/%d", n, len(src))
			return 0
		}

		// Uncompress the data.
		buf := make([]byte, len(src))
		n, err = lz4.UncompressBlock(zbuf, buf)
		if err != nil {
			t.Fatal(err)
		}
		buf = buf[:n]
		if !reflect.DeepEqual(src, buf) {
			t.Error("uncompressed compressed data not matching initial input")
			return 0
		}

		return len(zbuf)
	}

	for _, tc := range rawFiles {
		src, err := ioutil.ReadFile(tc.file)
		if err != nil {
			t.Fatal(err)
		}
		tc.src = src

		var n, nhc int
		t.Run("", func(t *testing.T) {
			tc := tc
			t.Run(tc.file, func(t *testing.T) {
				t.Parallel()
				n = run(tc, func(src, dst []byte) (int, error) {
					var ht [1 << 16]int
					return lz4.CompressBlock(src, dst, ht[:])
				})
			})
			t.Run(fmt.Sprintf("%s HC", tc.file), func(t *testing.T) {
				t.Parallel()
				nhc = run(tc, func(src, dst []byte) (int, error) {
					return lz4.CompressBlockHC(src, dst, -1)
				})
			})
		})
		fmt.Printf("%-40s: %8d / %8d / %8d\n", tc.file, n, nhc, len(src))
	}
}
