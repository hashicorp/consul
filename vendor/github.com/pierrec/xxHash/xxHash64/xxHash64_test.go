package xxHash64_test

import (
	"encoding/binary"
	"hash/crc64"
	"hash/fnv"
	"testing"

	"github.com/pierrec/xxHash/xxHash64"
)

type test struct {
	sum             uint64
	data, printable string
}

var testdata = []test{
	{0xef46db3751d8e999, "", ""},
	{0xd24ec4f1a98c6e5b, "a", ""},
	{0x65f708ca92d04a61, "ab", ""},
	{0x44bc2cf5ad770999, "abc", ""},
	{0xde0327b0d25d92cc, "abcd", ""},
	{0x07e3670c0c8dc7eb, "abcde", ""},
	{0xfa8afd82c423144d, "abcdef", ""},
	{0x1860940e2902822d, "abcdefg", ""},
	{0x3ad351775b4634b7, "abcdefgh", ""},
	{0x27f1a34fdbb95e13, "abcdefghi", ""},
	{0xd6287a1de5498bb2, "abcdefghij", ""},
	{0xbf2cd639b4143b80, "abcdefghijklmnopqrstuvwxyz012345", ""},
	{0x64f23ecf1609b766, "abcdefghijklmnopqrstuvwxyz0123456789", ""},
	{0xc5a8b11443765630, "Lorem ipsum dolor sit amet, consectetur adipiscing elit, sed do eiusmod tempor incididunt ut labore et dolore magna aliqua. Ut enim ad minim veniam, quis nostrud exercitation ullamco laboris nisi ut aliquip ex ea commodo consequat. Duis aute irure dolor in reprehenderit in voluptate velit esse cillum dolore eu fugiat nulla pariatur. Excepteur sint occaecat cupidatat non proident, sunt in culpa qui officia deserunt mollit anim id est laborum.", ""},
}

func init() {
	for i := range testdata {
		d := &testdata[i]
		if len(d.data) > 20 {
			d.printable = d.data[:20]
		} else {
			d.printable = d.data
		}
	}
}

func TestBlockSize(t *testing.T) {
	xxh := xxHash64.New(0)
	if s := xxh.BlockSize(); s <= 0 {
		t.Errorf("invalid BlockSize: %d", s)
	}
}

func TestSize(t *testing.T) {
	xxh := xxHash64.New(0)
	if s := xxh.Size(); s != 8 {
		t.Errorf("invalid Size: got %d expected 8", s)
	}
}

func TestData(t *testing.T) {
	for i, td := range testdata {
		xxh := xxHash64.New(0)
		data := []byte(td.data)
		xxh.Write(data)
		if h := xxh.Sum64(); h != td.sum {
			t.Errorf("test %d: xxh64(%s)=0x%x expected 0x%x", i, td.printable, h, td.sum)
			t.FailNow()
		}
		if h := xxHash64.Checksum(data, 0); h != td.sum {
			t.Errorf("test %d: xxh64(%s)=0x%x expected 0x%x", i, td.printable, h, td.sum)
			t.FailNow()
		}
	}
}

func TestSplitData(t *testing.T) {
	for i, td := range testdata {
		xxh := xxHash64.New(0)
		data := []byte(td.data)
		l := len(data) / 2
		xxh.Write(data[0:l])
		xxh.Write(data[l:])
		h := xxh.Sum64()
		if h != td.sum {
			t.Errorf("test %d: xxh64(%s)=0x%x expected 0x%x", i, td.printable, h, td.sum)
			t.FailNow()
		}
	}
}

func TestSum(t *testing.T) {
	for i, td := range testdata {
		xxh := xxHash64.New(0)
		data := []byte(td.data)
		xxh.Write(data)
		b := xxh.Sum(data)
		if h := binary.LittleEndian.Uint64(b[len(data):]); h != td.sum {
			t.Errorf("test %d: xxh64(%s)=0x%x expected 0x%x", i, td.printable, h, td.sum)
			t.FailNow()
		}
	}
}

func TestReset(t *testing.T) {
	xxh := xxHash64.New(0)
	for i, td := range testdata {
		xxh.Write([]byte(td.data))
		h := xxh.Sum64()
		if h != td.sum {
			t.Errorf("test %d: xxh64(%s)=0x%x expected 0x%x", i, td.printable, h, td.sum)
			t.FailNow()
		}
		xxh.Reset()
	}
}

///////////////////////////////////////////////////////////////////////////////
// Benchmarks
//
var testdata1 = []byte(testdata[len(testdata)-1].data)

func Benchmark_XXH64(b *testing.B) {
	h := xxHash64.New(0)
	for n := 0; n < b.N; n++ {
		h.Write(testdata1)
		h.Sum64()
		h.Reset()
	}
}

func Benchmark_XXH64_Checksum(b *testing.B) {
	for n := 0; n < b.N; n++ {
		xxHash64.Checksum(testdata1, 0)
	}
}

func Benchmark_CRC64(b *testing.B) {
	t := crc64.MakeTable(0)
	for i := 0; i < b.N; i++ {
		crc64.Checksum(testdata1, t)
	}
}

func Benchmark_Fnv64(b *testing.B) {
	h := fnv.New64()
	for i := 0; i < b.N; i++ {
		h.Write(testdata1)
		h.Sum64()
		h.Reset()
	}
}
