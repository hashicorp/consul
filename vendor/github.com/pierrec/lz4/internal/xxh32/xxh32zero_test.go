package xxh32_test

import (
	"encoding/binary"
	"hash/crc32"
	"hash/fnv"
	"testing"

	"github.com/pierrec/lz4/internal/xxh32"
)

type test struct {
	sum             uint32
	data, printable string
}

var testdata = []test{
	// {0x02cc5d05, "", ""},
	// {0x550d7456, "a", ""},
	// {0x4999fc53, "ab", ""},
	// {0x32d153ff, "abc", ""},
	// {0xa3643705, "abcd", ""},
	// {0x9738f19b, "abcde", ""},
	// {0x8b7cd587, "abcdef", ""},
	// {0x9dd093b3, "abcdefg", ""},
	// {0x0bb3c6bb, "abcdefgh", ""},
	// {0xd03c13fd, "abcdefghi", ""},
	// {0x8b988cfe, "abcdefghij", ""},
	{0x9d2d8b62, "abcdefghijklmnop", ""},
	// {0x42ae804d, "abcdefghijklmnopqrstuvwxyz0123456789", ""},
	// {0x62b4ed00, "Lorem ipsum dolor sit amet, consectetur adipiscing elit, sed do eiusmod tempor incididunt ut labore et dolore magna aliqua. Ut enim ad minim veniam, quis nostrud exercitation ullamco laboris nisi ut aliquip ex ea commodo consequat. Duis aute irure dolor in reprehenderit in voluptate velit esse cillum dolore eu fugiat nulla pariatur. Excepteur sint occaecat cupidatat non proident, sunt in culpa qui officia deserunt mollit anim id est laborum.", ""},
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

func TestZeroBlockSize(t *testing.T) {
	var xxh xxh32.XXHZero
	if s := xxh.BlockSize(); s <= 0 {
		t.Errorf("invalid BlockSize: %d", s)
	}
}

func TestZeroSize(t *testing.T) {
	var xxh xxh32.XXHZero
	if s := xxh.Size(); s != 4 {
		t.Errorf("invalid Size: got %d expected 4", s)
	}
}

func TestZeroData(t *testing.T) {
	for i, td := range testdata {
		var xxh xxh32.XXHZero
		data := []byte(td.data)
		xxh.Write(data)
		if h := xxh.Sum32(); h != td.sum {
			t.Errorf("test %d: xxh32(%s)=0x%x expected 0x%x", i, td.printable, h, td.sum)
			t.FailNow()
		}
		if h := xxh32.ChecksumZero(data); h != td.sum {
			t.Errorf("test %d: xxh32(%s)=0x%x expected 0x%x", i, td.printable, h, td.sum)
			t.FailNow()
		}
	}
}

func TestZeroSplitData(t *testing.T) {
	for i, td := range testdata {
		var xxh xxh32.XXHZero
		data := []byte(td.data)
		l := len(data) / 2
		xxh.Write(data[0:l])
		xxh.Write(data[l:])
		h := xxh.Sum32()
		if h != td.sum {
			t.Errorf("test %d: xxh32(%s)=0x%x expected 0x%x", i, td.printable, h, td.sum)
			t.FailNow()
		}
	}
}

func TestZeroSum(t *testing.T) {
	for i, td := range testdata {
		var xxh xxh32.XXHZero
		data := []byte(td.data)
		xxh.Write(data)
		b := xxh.Sum(data)
		if h := binary.LittleEndian.Uint32(b[len(data):]); h != td.sum {
			t.Errorf("test %d: xxh32(%s)=0x%x expected 0x%x", i, td.printable, h, td.sum)
			t.FailNow()
		}
	}
}

func TestZeroChecksum(t *testing.T) {
	for i, td := range testdata {
		data := []byte(td.data)
		h := xxh32.ChecksumZero(data)
		if h != td.sum {
			t.Errorf("test %d: xxh32(%s)=0x%x expected 0x%x", i, td.printable, h, td.sum)
			t.FailNow()
		}
	}
}

func TestZeroReset(t *testing.T) {
	var xxh xxh32.XXHZero
	for i, td := range testdata {
		xxh.Write([]byte(td.data))
		h := xxh.Sum32()
		if h != td.sum {
			t.Errorf("test %d: xxh32(%s)=0x%x expected 0x%x", i, td.data[:40], h, td.sum)
			t.FailNow()
		}
		xxh.Reset()
	}
}

///////////////////////////////////////////////////////////////////////////////
// Benchmarks
//
var testdata1 = []byte(testdata[len(testdata)-1].data)

func Benchmark_XXH32(b *testing.B) {
	var h xxh32.XXHZero
	for n := 0; n < b.N; n++ {
		h.Write(testdata1)
		h.Sum32()
		h.Reset()
	}
}

func Benchmark_XXH32_Checksum(b *testing.B) {
	for n := 0; n < b.N; n++ {
		xxh32.ChecksumZero(testdata1)
	}
}

func Benchmark_CRC32(b *testing.B) {
	t := crc32.MakeTable(0)
	for i := 0; i < b.N; i++ {
		crc32.Checksum(testdata1, t)
	}
}

func Benchmark_Fnv32(b *testing.B) {
	h := fnv.New32()
	for i := 0; i < b.N; i++ {
		h.Write(testdata1)
		h.Sum32()
		h.Reset()
	}
}
