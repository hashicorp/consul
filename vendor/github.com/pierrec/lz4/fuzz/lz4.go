package lz4

import (
	"bytes"
	"io/ioutil"

	"github.com/pierrec/lz4"
)

// lz4.Reader fuzz function
func Fuzz(data []byte) int {
	// uncompress some data
	d, err := ioutil.ReadAll(lz4.NewReader(bytes.NewReader(data)))
	if err != nil {
		return 0
	}

	// got valid compressed data
	// compress the uncompressed data
	// and compare with the original input
	buf := bytes.NewBuffer(nil)
	zw := lz4.NewWriter(buf)
	n, err := zw.Write(d)
	if err != nil {
		panic(err)
	}
	if n != len(d) {
		panic("short write")
	}
	err = zw.Close()
	if err != nil {
		panic(err)
	}

	// uncompress the newly compressed data
	ud, err := ioutil.ReadAll(lz4.NewReader(buf))
	if err != nil {
		panic(err)
	}
	if bytes.Compare(d, ud) != 0 {
		panic("not equal")
	}

	return 1
}
