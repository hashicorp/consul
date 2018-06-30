package lz4_test

import (
	"bytes"
	"io"
	"io/ioutil"
	"testing"

	"github.com/pierrec/lz4"
)

func BenchmarkCompress(b *testing.B) {
	var hashTable [1 << 16]int
	buf := make([]byte, len(pg1661))

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		lz4.CompressBlock(pg1661, buf, hashTable[:])
	}
}

func BenchmarkCompressHC(b *testing.B) {
	buf := make([]byte, len(pg1661))

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		lz4.CompressBlockHC(pg1661, buf, 16)
	}
}

func BenchmarkUncompress(b *testing.B) {
	buf := make([]byte, len(pg1661))

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		lz4.UncompressBlock(pg1661LZ4, buf)
	}
}

func mustLoadFile(f string) []byte {
	b, err := ioutil.ReadFile(f)
	if err != nil {
		panic(err)
	}
	return b
}

var (
	pg1661    = mustLoadFile("testdata/pg1661.txt")
	digits    = mustLoadFile("testdata/e.txt")
	twain     = mustLoadFile("testdata/Mark.Twain-Tom.Sawyer.txt")
	random    = mustLoadFile("testdata/random.data")
	pg1661LZ4 = mustLoadFile("testdata/pg1661.txt.lz4")
	digitsLZ4 = mustLoadFile("testdata/e.txt.lz4")
	twainLZ4  = mustLoadFile("testdata/Mark.Twain-Tom.Sawyer.txt.lz4")
	randomLZ4 = mustLoadFile("testdata/random.data.lz4")
)

func benchmarkUncompress(b *testing.B, compressed []byte) {
	r := bytes.NewReader(compressed)
	zr := lz4.NewReader(r)

	// Determine the uncompressed size of testfile.
	uncompressedSize, err := io.Copy(ioutil.Discard, zr)
	if err != nil {
		b.Fatal(err)
	}

	b.SetBytes(uncompressedSize)
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		r.Reset(compressed)
		zr.Reset(r)
		io.Copy(ioutil.Discard, zr)
	}
}

func BenchmarkUncompressPg1661(b *testing.B) { benchmarkUncompress(b, pg1661LZ4) }
func BenchmarkUncompressDigits(b *testing.B) { benchmarkUncompress(b, digitsLZ4) }
func BenchmarkUncompressTwain(b *testing.B)  { benchmarkUncompress(b, twainLZ4) }
func BenchmarkUncompressRand(b *testing.B)   { benchmarkUncompress(b, randomLZ4) }

func benchmarkCompress(b *testing.B, uncompressed []byte) {
	w := bytes.NewBuffer(nil)
	zw := lz4.NewWriter(w)
	r := bytes.NewReader(uncompressed)

	// Determine the compressed size of testfile.
	compressedSize, err := io.Copy(zw, r)
	if err != nil {
		b.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		b.Fatal(err)
	}

	b.SetBytes(compressedSize)
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		r.Reset(uncompressed)
		zw.Reset(w)
		io.Copy(zw, r)
	}
}

func BenchmarkCompressPg1661(b *testing.B) { benchmarkCompress(b, pg1661) }
func BenchmarkCompressDigits(b *testing.B) { benchmarkCompress(b, digits) }
func BenchmarkCompressTwain(b *testing.B)  { benchmarkCompress(b, twain) }
func BenchmarkCompressRand(b *testing.B)   { benchmarkCompress(b, random) }
