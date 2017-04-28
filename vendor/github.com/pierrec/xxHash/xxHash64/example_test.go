package xxHash64_test

import (
	"bytes"
	"fmt"
	"github.com/pierrec/xxHash/xxHash64"
)

func ExampleNew() {
	buf := bytes.NewBufferString("this is a test")
	x := xxHash64.New(0xCAFE)
	x.Write(buf.Bytes())
	fmt.Printf("%x\n", x.Sum64())
	// Output: 4228c3215949e862
}

func ExampleChecksum() {
	buf := bytes.NewBufferString("this is a test")
	fmt.Printf("%x\n", xxHash64.Checksum(buf.Bytes(), 0xCAFE))
	// Output: 4228c3215949e862
}
