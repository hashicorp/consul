package lz4_test

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"io"
	"io/ioutil"
	"math/big"
	"os"
	"reflect"
	"testing"

	"github.com/pierrec/lz4"
)

// testBuffer wraps bytes.Buffer to remove the WriteTo() and ReadFrom() methods.
type testBuffer struct {
	buf *bytes.Buffer
}

func (b *testBuffer) Read(buf []byte) (int, error) {
	return b.buf.Read(buf)
}

func (b *testBuffer) Write(buf []byte) (int, error) {
	return b.buf.Write(buf)
}

func (b *testBuffer) Len() int {
	return b.buf.Len()
}

func (b *testBuffer) Bytes() []byte {
	return b.buf.Bytes()
}

// testData represents a test data item. It is really used to provide a human readable label to a slice of bytes.
type testData struct {
	label string
	data  []byte
}

// testHeader represents a test data item. It is really used to provide a human readable label to an LZ4 header.
type testHeader struct {
	label  string
	header lz4.Header
}

// compareHeaders... compares 2 lz4 headers.
func compareHeaders(h, hh lz4.Header, t *testing.T) {
	ok := true
	if h.BlockDependency != hh.BlockDependency {
		t.Errorf("BlockDependency: expected %v, got %v", h.BlockDependency, hh.BlockDependency)
		ok = false
	}
	if h.BlockChecksum != hh.BlockChecksum {
		t.Errorf("BlockChecksum: expected %v, got %v", h.BlockChecksum, hh.BlockChecksum)
		ok = false
	}
	if h.NoChecksum != hh.NoChecksum {
		t.Errorf("NoChecksum: expected %v, got %v", h.NoChecksum, hh.NoChecksum)
		ok = false
	}
	if h.BlockMaxSize != hh.BlockMaxSize {
		t.Errorf("BlockMaxSize: expected %d, got %d", h.BlockMaxSize, hh.BlockMaxSize)
		ok = false
	}
	if h.Size != hh.Size {
		t.Errorf("Size: expected %d, got %d", h.Size, hh.Size)
		ok = false
	}
	// 	if h.Dict != hh.Dict {
	// 		t.Errorf("Dict: expected %d, got %d", h.Dict, hh.Dict)
	// 		ok = false
	// 	}
	// 	if h.DictID != hh.DictID {
	// 		t.Errorf("DictID: expected %d, got %d", h.DictID, hh.DictID)
	// 		ok = false
	// 	}
	if !ok {
		t.FailNow()
	}
}

var (
	lorem = []byte("Lorem ipsum dolor sit amet, consectetur adipiscing elit, sed do eiusmod tempor incididunt ut labore et dolore magna aliqua. Ut enim ad minim veniam, quis nostrud exercitation ullamco laboris nisi ut aliquip ex ea commodo consequat. Duis aute irure dolor in reprehenderit in voluptate velit esse cillum dolore eu fugiat nulla pariatur. Excepteur sint occaecat cupidatat non proident, sunt in culpa qui officia deserunt mollit anim id est laborum.")
	// Initial data items used for testing. More are added with random and other kind of data.
	testDataItems = []testData{
		{"empty", nil},
		{
			"small pattern",
			[]byte("aaaaaaaaaaaaaaaaaaa"),
		},
		{
			"small pattern long",
			[]byte("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"),
		},
		{
			"medium pattern",
			[]byte("abcdefghijklmnopqabcdefghijklmnopq"),
		},
		{
			"lorem",
			lorem,
		},
	}
	testHeaderItems = []testHeader{}
)

// Build the list of all possible headers with the default values + the ones defined in the map.
func buildHeaders(options map[string][]interface{}) []testHeader {
	testHeaderItems := make([]testHeader, 1)
	for fieldName, fieldData := range options {
		for _, o := range fieldData {
			for _, d := range testHeaderItems {
				s := reflect.ValueOf(&d.header).Elem()
				t := s.Type()
				for i := 0; i < s.NumField(); i++ {
					if t.Field(i).Name == fieldName {
						switch f := s.Field(i); f.Kind() {
						case reflect.Bool:
							f.SetBool(o.(bool))
						case reflect.Int:
							f.SetInt(int64(o.(int)))
						case reflect.Int64:
							switch o.(type) {
							case int:
								f.SetInt(int64(o.(int)))
							default:
								f.SetInt(o.(int64))
							}
						case reflect.Uint32:
							switch o.(type) {
							case int:
								f.SetUint(uint64(o.(int)))
							default:
								f.SetUint(uint64(o.(uint32)))
							}
						case reflect.Uint64:
							switch o.(type) {
							case int:
								f.SetUint(uint64(o.(int)))
							default:
								f.SetUint(o.(uint64))
							}
						default:
							panic(fmt.Sprintf("unsupported type: %v", f.Kind()))
						}
						d.label = fmt.Sprintf("%+v", d.header)
						testHeaderItems = append(testHeaderItems, d)
						break
					}
				}
			}
		}
	}

	for i, n := 0, len(testHeaderItems); i < n; {
		testHeaderItem := testHeaderItems[i]
		// remove the 0 BlockMaxSize value as it is invalid and we have provisioned all possible values already.
		if testHeaderItem.header.BlockMaxSize == 0 {
			n--
			testHeaderItems[i], testHeaderItems = testHeaderItems[n], testHeaderItems[:n]
		} else {
			testHeaderItem.label = fmt.Sprintf("%+v", testHeaderItem)
			i++
		}
	}

	return testHeaderItems
}

// Generate all possible LZ4 headers.
func init() {
	// Only set the relevant headers having an impact on the comrpession.
	seed := map[string][]interface{}{
		"BlockDependency": {true},
		"BlockChecksum":   {true},
		"NoChecksum":      {true},
		// "Dict":            {true},
		// Enabling this substantially increase the testing time.
		// As this test is not really required it is disabled.
		// "HighCompression": {true},
	}
	for _, bms := range lz4.BlockMaxSizeItems {
		seed["BlockMaxSize"] = append(seed["BlockMaxSize"], bms)
	}
	testHeaderItems = buildHeaders(seed)
}

// Initialize the test data with various sizes of uncompressible and compressible data.
func init() {
	maxSize := 10 << 20 // > max block max size of 4Mb

	// repeated data with very high compression ratio
	repeat := make([]byte, maxSize)
	for i := copy(repeat, lorem); i < len(repeat); {
		i += copy(repeat[i:], repeat[:i])
	}

	// repeated data with small compression ratio
	repeatlow := make([]byte, maxSize)
	for i := 0; i < len(repeatlow); {
		i += copy(repeatlow[i:], lorem)
		// randomly skip some bytes to make sure the pattern does not repeat too much
		n, _ := rand.Int(rand.Reader, big.NewInt(int64(10)))
		i += int(n.Int64())
	}

	// random data: low to no compression
	random := make([]byte, maxSize)
	if _, err := rand.Read(random); err != nil {
		panic(fmt.Sprintf("cannot initialize random data for size %d", maxSize))
	}

	// generate some test data with various sizes and kind of data: all valid block max sizes + others
	for _, size := range lz4.BlockMaxSizeItems {
		testDataItems = append(
			testDataItems,
			testData{fmt.Sprintf("random %d", size), random[:size]},
			testData{fmt.Sprintf("random < %d", size), random[:size/3]},
			testData{fmt.Sprintf("repeated %d", size), repeat[:size]},
			testData{fmt.Sprintf("repeated < %d", size), repeat[:size/3]},
		)
	}
}

// Test low levels core functions:
// a. compress and compare with supplied data if any
// b. decompress the previous data and compare it with the original one
func TestBlock(t *testing.T) {
	for _, compress := range []func([]byte, []byte, int) (int, error){
		lz4.CompressBlock,
		lz4.CompressBlockHC,
	} {
		for _, item := range testDataItems {
			data := item.data
			z := make([]byte, lz4.CompressBlockBound(len(data)))
			n, err := compress(data, z, 0)
			if n == 0 { // not compressible
				continue
			}
			if err != nil {
				t.Errorf("CompressBlock: %s", err)
				t.FailNow()
			}
			z = z[:n]
			d := make([]byte, len(data))
			n, err = lz4.UncompressBlock(z, d, 0)
			if err != nil {
				t.Errorf("UncompressBlock: %s", err)
				t.FailNow()
			}
			d = d[:n]
			if !bytes.Equal(d, data) {
				t.Errorf("invalid decompressed data: %s: %s", item.label, string(d))
				t.FailNow()
			}
		}
	}
}

func TestBlockCompression(t *testing.T) {
	input := make([]byte, 64*1024)

	for i := 0; i < 64*1024; i += 1 {
		input[i] = byte(i & 0x1)
	}
	output := make([]byte, 64*1024)

	c, err := lz4.CompressBlock(input, output, 0)

	if err != nil {
		t.Fatal(err)
	}

	if c == 0 {
		t.Fatal("cannot compress compressible data")
	}
}

func BenchmarkUncompressBlock(b *testing.B) {
	d := make([]byte, len(lorem))
	z := make([]byte, len(lorem))
	n, err := lz4.CompressBlock(lorem, z, 0)
	if err != nil {
		b.Errorf("CompressBlock: %s", err)
		b.FailNow()
	}
	z = z[:n]
	for i := 0; i < b.N; i++ {
		lz4.UncompressBlock(z, d, 0)
	}
}

func BenchmarkUncompressConstantBlock(b *testing.B) {
	d := make([]byte, 4096)
	z := make([]byte, 4096)
	source := make([]byte, 4096)
	n, err := lz4.CompressBlock(source, z, 0)
	if err != nil {
		b.Errorf("CompressBlock: %s", err)
		b.FailNow()
	}
	z = z[:n]
	for i := 0; i < b.N; i++ {
		lz4.UncompressBlock(z, d, 0)
	}
}

func BenchmarkCompressBlock(b *testing.B) {
	d := append([]byte{}, lorem...)
	z := make([]byte, len(lorem))
	n, err := lz4.CompressBlock(d, z, 0)
	if err != nil {
		b.Errorf("CompressBlock: %s", err)
		b.FailNow()
	}
	z = z[:n]
	for i := 0; i < b.N; i++ {
		d = append([]byte{}, lorem...)
		lz4.CompressBlock(d, z, 0)
	}
}

func BenchmarkCompressConstantBlock(b *testing.B) {
	d := make([]byte, 4096)
	z := make([]byte, 4096)
	n, err := lz4.CompressBlock(d, z, 0)
	if err != nil {
		b.Errorf("CompressBlock: %s", err)
		b.FailNow()
	}
	z = z[:n]
	for i := 0; i < b.N; i++ {
		lz4.CompressBlock(d, z, 0)
	}
}

func BenchmarkCompressBlockHC(b *testing.B) {
	d := append([]byte{}, lorem...)
	z := make([]byte, len(lorem))
	n, err := lz4.CompressBlockHC(d, z, 0)
	if err != nil {
		b.Errorf("CompressBlock: %s", err)
		b.FailNow()
	}
	z = z[:n]
	for i := 0; i < b.N; i++ {
		d = append([]byte{}, lorem...)
		lz4.CompressBlockHC(d, z, 0)
	}
}
func BenchmarkCompressEndToEnd(b *testing.B) {
	w := lz4.NewWriter(ioutil.Discard)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := w.Write(lorem); err != nil {
			b.Fatal(err)
		}
	}
}

// TestNoWrite compresses without any call to Write() (empty frame).
// It does so checking all possible headers.
func TestNoWrite(t *testing.T) {
	// that is 2*2*2*2*2*2^4 = 512 headers!
	seed := map[string][]interface{}{
		"BlockDependency": {true},
		"BlockChecksum":   {true},
		"NoChecksum":      {true},
		"Size":            {999},
		// "Dict":            {true},
		// Enabling this substantially increase the testing time.
		// As this test is not really required it is disabled.
		// "HighCompression": {true},
	}
	for _, bms := range lz4.BlockMaxSizeItems {
		seed["BlockMaxSize"] = append(seed["BlockMaxSize"], bms)
	}
	testHeaderItems := buildHeaders(seed)

	for _, h := range testHeaderItems {
		rw := bytes.NewBuffer(nil)

		w := lz4.NewWriter(rw)
		w.Header = h.header
		if err := w.Close(); err != nil {
			t.Errorf("Close(): unexpected error: %v", err)
			t.FailNow()
		}

		r := lz4.NewReader(rw)
		n, err := r.Read(nil)
		if err != nil {
			t.Errorf("Read(): unexpected error: %v", err)
			t.FailNow()
		}
		if n != 0 {
			t.Errorf("expected 0 bytes read, got %d", n)
			t.FailNow()
		}

		buf := make([]byte, 16)
		n, err = r.Read(buf)
		if err != nil && err != io.EOF {
			t.Errorf("Read(): unexpected error: %v", err)
			t.FailNow()
		}
		if n != 0 {
			t.Errorf("expected 0 bytes read, got %d", n)
			t.FailNow()
		}
	}
}

// TestReset tests that the Reset() method resets the header on the Reader and Writer.
func TestReset(t *testing.T) {
	h := lz4.Header{
		BlockDependency: true,
		BlockChecksum:   true,
		NoChecksum:      true,
		BlockMaxSize:    123,
		Size:            999,
		// Dict:            true,
		// DictID:          555,
	}
	dh := lz4.Header{}

	w := lz4.NewWriter(nil)
	w.Header = h
	w.Reset(nil)
	compareHeaders(w.Header, dh, t)

	r := lz4.NewReader(nil)
	r.Header = h
	r.Reset(nil)
	compareHeaders(r.Header, dh, t)
}

// TestFrame compresses and decompresses LZ4 streams with various input data and options.
func TestFrame(t *testing.T) {
	for _, tdata := range testDataItems {
		data := tdata.data
		t.Run(tdata.label, func(t *testing.T) {
			t.Parallel()
			// test various options
			for _, headerItem := range testHeaderItems {
				tag := tdata.label + ": " + headerItem.label
				rw := bytes.NewBuffer(nil)

				// Set all options to non default values and compress
				w := lz4.NewWriter(rw)
				w.Header = headerItem.header

				n, err := w.Write(data)
				if err != nil {
					t.Errorf("%s: Write(): unexpected error: %v", tag, err)
					t.FailNow()
				}
				if n != len(data) {
					t.Errorf("%s: Write(): expected %d bytes written, got %d", tag, len(data), n)
					t.FailNow()
				}
				if err = w.Close(); err != nil {
					t.Errorf("%s: Close(): unexpected error: %v", tag, err)
					t.FailNow()
				}

				// Decompress
				r := lz4.NewReader(rw)
				n, err = r.Read(nil)
				if err != nil {
					t.Errorf("%s: Read(): unexpected error: %v", tag, err)
					t.FailNow()
				}
				if n != 0 {
					t.Errorf("%s: Read(): expected 0 bytes read, got %d", tag, n)
				}

				buf := make([]byte, len(data))
				n, err = r.Read(buf)
				if err != nil && err != io.EOF {
					t.Errorf("%s: Read(): unexpected error: %v", tag, err)
					t.FailNow()
				}
				if n != len(data) {
					t.Errorf("%s: Read(): expected %d bytes read, got %d", tag, len(data), n)
				}
				buf = buf[:n]
				if !bytes.Equal(buf, data) {
					t.Errorf("%s: decompress(compress(data)) != data (%d/%d)", tag, len(buf), len(data))
					t.FailNow()
				}

				compareHeaders(w.Header, r.Header, t)
			}
		})
	}
}

// TestReadFromWriteTo tests the Reader.WriteTo() and Writer.ReadFrom() methods.
func TestReadFromWriteTo(t *testing.T) {
	for _, tdata := range testDataItems {
		data := tdata.data

		t.Run(tdata.label, func(t *testing.T) {
			t.Parallel()
			// test various options
			for _, headerItem := range testHeaderItems {
				tag := "ReadFromWriteTo: " + tdata.label + ": " + headerItem.label
				dbuf := bytes.NewBuffer(data)

				zbuf := bytes.NewBuffer(nil)
				w := lz4.NewWriter(zbuf)
				w.Header = headerItem.header
				if _, err := w.ReadFrom(dbuf); err != nil {
					t.Errorf("%s: unexpected error: %s", tag, err)
					t.FailNow()
				}

				if err := w.Close(); err != nil {
					t.Errorf("%s: unexpected error: %s", tag, err)
					t.FailNow()
				}

				buf := bytes.NewBuffer(nil)
				r := lz4.NewReader(zbuf)
				if _, err := r.WriteTo(buf); err != nil {
					t.Errorf("%s: unexpected error: %s", tag, err)
					t.FailNow()
				}

				if !bytes.Equal(buf.Bytes(), data) {
					t.Errorf("%s: decompress(compress(data)) != data (%d/%d)", tag, buf.Len(), len(data))
					t.FailNow()
				}
			}
		})
	}
}

// TestCopy will use io.Copy and avoid using Reader.WriteTo() and Writer.ReadFrom().
func TestCopy(t *testing.T) {
	for _, tdata := range testDataItems {
		data := tdata.data
		t.Run(tdata.label, func(t *testing.T) {
			t.Parallel()

			w := lz4.NewWriter(nil)
			r := lz4.NewReader(nil)
			// test various options
			for _, headerItem := range testHeaderItems {
				tag := "io.Copy: " + tdata.label + ": " + headerItem.label
				dbuf := &testBuffer{bytes.NewBuffer(data)}

				zbuf := bytes.NewBuffer(nil)
				w.Reset(zbuf)
				w.Header = headerItem.header
				if _, err := io.Copy(w, dbuf); err != nil {
					t.Errorf("%s: unexpected error: %s", tag, err)
					t.FailNow()
				}

				if err := w.Close(); err != nil {
					t.Errorf("%s: unexpected error: %s", tag, err)
					t.FailNow()
				}

				buf := &testBuffer{bytes.NewBuffer(nil)}
				r.Reset(zbuf)
				if _, err := io.Copy(buf, r); err != nil {
					t.Errorf("%s: unexpected error: %s", tag, err)
					t.FailNow()
				}

				if !bytes.Equal(buf.Bytes(), data) {
					t.Errorf("%s: decompress(compress(data)) != data (%d/%d)", tag, buf.Len(), len(data))
					t.FailNow()
				}
			}
		})
	}
}

func TestSkippable(t *testing.T) {
	w := lz4.NewWriter(nil)
	r := lz4.NewReader(nil)

	skippable := make([]byte, 1<<20)
	binary.LittleEndian.PutUint32(skippable, lz4.FrameSkipMagic)
	binary.LittleEndian.PutUint32(skippable[4:], uint32(len(skippable)-8))

	buf := make([]byte, len(lorem))

	tag := "skippable first"
	zbuf := bytes.NewBuffer(skippable)
	w.Reset(zbuf)
	w.Write(lorem)
	w.Close()

	r.Reset(zbuf)
	if _, err := r.Read(buf); err != nil {
		t.Errorf("%s: unexpected error: %s", tag, err)
		t.FailNow()
	}

	tag = "skippable last"
	zbuf = bytes.NewBuffer(nil)
	w.Reset(zbuf)
	w.Write(lorem)
	w.Close()
	zbuf.Write(skippable)

	r.Reset(zbuf)
	if _, err := r.Read(buf); err != nil {
		t.Errorf("%s: unexpected error: %s", tag, err)
		t.FailNow()
	}

	tag = "skippable middle"
	zbuf = bytes.NewBuffer(nil)
	w.Reset(zbuf)
	w.Write(lorem)
	zbuf.Write(skippable)
	w.Write(lorem)
	w.Close()

	r.Reset(zbuf)
	if _, err := r.Read(buf); err != nil {
		t.Errorf("%s: unexpected error: %s", tag, err)
		t.FailNow()
	}

}

func TestWrittenCountAfterBufferedWrite(t *testing.T) {
	w := lz4.NewWriter(bytes.NewBuffer(nil))
	w.Header.BlockDependency = true

	if n, _ := w.Write([]byte{1}); n != 1 {
		t.Errorf("expected to write 1 byte, wrote %d", n)
		t.FailNow()
	}

	forcesWrite := make([]byte, 1<<16)

	if n, _ := w.Write(forcesWrite); n != len(forcesWrite) {
		t.Errorf("expected to write %d bytes, wrote %d", len(forcesWrite), n)
		t.FailNow()
	}
}

func TestWrittenBlocksExactlyWindowSize(t *testing.T) {
	input := make([]byte, 128*1024)

	copy(input[64*1024-1:], []byte{1, 2, 3, 4, 1, 2, 3, 4})

	output := writeReadChunked(t, input, 64*1024)

	if !bytes.Equal(input, output) {
		t.Errorf("output is not equal to source input")
		t.FailNow()
	}
}

func TestWrittenBlocksLessThanWindowSize(t *testing.T) {
	input := make([]byte, 80*1024)

	copy(input[64*1024-1:], []byte{1, 2, 3, 4, 1, 2, 3, 4})
	copy(input[72*1024-1:], []byte{5, 6, 7, 8, 5, 6, 7, 8})

	output := writeReadChunked(t, input, 8*1024)
	if !bytes.Equal(input, output) {
		t.Errorf("output is not equal to source input")
		t.FailNow()
	}
}

func writeReadChunked(t *testing.T, in []byte, chunkSize int) []byte {
	compressed := bytes.NewBuffer(nil)
	w := lz4.NewWriter(compressed)
	w.Header.BlockDependency = true

	buf := bytes.NewBuffer(in)
	for buf.Len() > 0 {
		_, err := w.Write(buf.Next(chunkSize))
		if err != nil {
			t.Errorf("unexpected error: %v", err)
			t.FailNow()
		}
	}

	r := lz4.NewReader(compressed)
	out := make([]byte, len(in))
	_, err := io.ReadFull(r, out)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
		t.FailNow()
	}
	return out
}

func TestMultiBlockWrite(t *testing.T) {
	f, err := os.Open("testdata/207326ba-36f8-11e7-954a-aca46ba8ca73.png")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	zbuf := bytes.NewBuffer(nil)
	zw := lz4.NewWriter(zbuf)
	if _, err := io.Copy(zw, f); err != nil {
		t.Fatal(err)
	}
	if err := zw.Flush(); err != nil {
		t.Fatal(err)
	}

	buf := bytes.NewBuffer(nil)
	zr := lz4.NewReader(zbuf)
	if _, err := io.Copy(buf, zr); err != nil {
		t.Fatal(err)
	}
}
