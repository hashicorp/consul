// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package adaptive

import (
	"bufio"
	"github.com/hashicorp/go-uuid"
	"math/rand"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestARTree_InsertAndSearchWords(t *testing.T) {
	t.Parallel()

	art := NewRadixTree[int]()

	file, err := os.Open("test-text/words.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	var lines []string

	scanner := bufio.NewScanner(file)
	// optionally, resize scanner's capacity for lines over 64K, see next example
	lineNumber := 1
	for scanner.Scan() {
		art.Insert(scanner.Bytes(), lineNumber)
		lineNumber += 1
		lines = append(lines, scanner.Text())
	}

	// optionally, resize scanner's capacity for lines over 64K, see next example
	lineNumber = 1
	for _, line := range lines {
		lineNumberFetched, f, _ := art.Get([]byte(line))
		require.True(t, f)
		require.Equal(t, lineNumberFetched, lineNumber)
		lineNumber += 1
	}

	artLeafMin := art.Minimum()
	artLeafMax := art.Maximum()
	require.Equal(t, artLeafMin.key, getTreeKey([]byte("A")))
	require.Equal(t, artLeafMax.key, getTreeKey([]byte("zythum")))
}

func TestARTree_InsertVeryLongKey(t *testing.T) {
	t.Parallel()

	key1 := []byte{16, 0, 0, 0, 7, 10, 0, 0, 0, 2, 17, 10, 0, 0, 0, 120, 10, 0, 0, 0, 120, 10, 0,
		0, 0, 216, 10, 0, 0, 0, 202, 10, 0, 0, 0, 194, 10, 0, 0, 0, 224, 10, 0, 0, 0,
		230, 10, 0, 0, 0, 210, 10, 0, 0, 0, 206, 10, 0, 0, 0, 208, 10, 0, 0, 0, 232,
		10, 0, 0, 0, 124, 10, 0, 0, 0, 124, 2, 16, 0, 0, 0, 2, 12, 185, 89, 44, 213,
		251, 173, 202, 211, 95, 185, 89, 110, 118, 251, 173, 202, 199, 101, 0,
		8, 18, 182, 92, 236, 147, 171, 101, 150, 195, 112, 185, 218, 108, 246,
		139, 164, 234, 195, 58, 177, 0, 8, 16, 0, 0, 0, 2, 12, 185, 89, 44, 213,
		251, 173, 202, 211, 95, 185, 89, 110, 118, 251, 173, 202, 199, 101, 0,
		8, 18, 180, 93, 46, 151, 9, 212, 190, 95, 102, 178, 217, 44, 178, 235,
		29, 190, 218, 8, 16, 0, 0, 0, 2, 12, 185, 89, 44, 213, 251, 173, 202,
		211, 95, 185, 89, 110, 118, 251, 173, 202, 199, 101, 0, 8, 18, 180, 93,
		46, 151, 9, 212, 190, 95, 102, 183, 219, 229, 214, 59, 125, 182, 71,
		108, 180, 220, 238, 150, 91, 117, 150, 201, 84, 183, 128, 8, 16, 0, 0,
		0, 2, 12, 185, 89, 44, 213, 251, 173, 202, 211, 95, 185, 89, 110, 118,
		251, 173, 202, 199, 101, 0, 8, 18, 180, 93, 46, 151, 9, 212, 190, 95,
		108, 176, 217, 47, 50, 219, 61, 134, 207, 97, 151, 88, 237, 246, 208,
		8, 18, 255, 255, 255, 219, 191, 198, 134, 5, 223, 212, 72, 44, 208,
		250, 180, 14, 1, 0, 0, 8}
	key2 := []byte{16, 0, 0, 0, 7, 10, 0, 0, 0, 2, 17, 10, 0, 0, 0, 120, 10, 0, 0, 0, 120, 10, 0,
		0, 0, 216, 10, 0, 0, 0, 202, 10, 0, 0, 0, 194, 10, 0, 0, 0, 224, 10, 0, 0, 0,
		230, 10, 0, 0, 0, 210, 10, 0, 0, 0, 206, 10, 0, 0, 0, 208, 10, 0, 0, 0, 232,
		10, 0, 0, 0, 124, 10, 0, 0, 0, 124, 2, 16, 0, 0, 0, 2, 12, 185, 89, 44, 213,
		251, 173, 202, 211, 95, 185, 89, 110, 118, 251, 173, 202, 199, 101, 0,
		8, 18, 182, 92, 236, 147, 171, 101, 150, 195, 112, 185, 218, 108, 246,
		139, 164, 234, 195, 58, 177, 0, 8, 16, 0, 0, 0, 2, 12, 185, 89, 44, 213,
		251, 173, 202, 211, 95, 185, 89, 110, 118, 251, 173, 202, 199, 101, 0,
		8, 18, 180, 93, 46, 151, 9, 212, 190, 95, 102, 178, 217, 44, 178, 235,
		29, 190, 218, 8, 16, 0, 0, 0, 2, 12, 185, 89, 44, 213, 251, 173, 202,
		211, 95, 185, 89, 110, 118, 251, 173, 202, 199, 101, 0, 8, 18, 180, 93,
		46, 151, 9, 212, 190, 95, 102, 183, 219, 229, 214, 59, 125, 182, 71,
		108, 180, 220, 238, 150, 91, 117, 150, 201, 84, 183, 128, 8, 16, 0, 0,
		0, 3, 12, 185, 89, 44, 213, 251, 133, 178, 195, 105, 183, 87, 237, 150,
		155, 165, 150, 229, 97, 182, 0, 8, 18, 161, 91, 239, 50, 10, 61, 150,
		223, 114, 179, 217, 64, 8, 12, 186, 219, 172, 150, 91, 53, 166, 221,
		101, 178, 0, 8, 18, 255, 255, 255, 219, 191, 198, 134, 5, 208, 212, 72,
		44, 208, 250, 180, 14, 1, 0, 0, 8}

	art := NewRadixTree[string]()
	val1 := art.Insert(key1, string(key1))
	val2 := art.Insert(key2, string(key2))
	require.Equal(t, val1, "")
	require.Equal(t, val2, "")

	art.Insert(key2, string(key2))
	require.Equal(t, art.size, uint64(2))
}

func TestARTree_InsertSearchAndDelete(t *testing.T) {
	t.Parallel()

	art := NewRadixTree[int]()

	file, err := os.Open("test-text/words.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	var lines []string

	scanner := bufio.NewScanner(file)
	// optionally, resize scanner's capacity for lines over 64K, see next example
	lineNumber := 1
	for scanner.Scan() {
		art.Insert(scanner.Bytes(), lineNumber)
		lineNumber += 1
		lines = append(lines, scanner.Text())
	}

	// optionally, resize scanner's capacity for lines over 64K, see next example
	lineNumber = 1
	for _, line := range lines {
		lineNumberFetched, f, _ := art.Get([]byte(line))
		require.True(t, f)
		require.Equal(t, lineNumberFetched, lineNumber)
		val := art.Delete([]byte(line))
		require.Equal(t, val, lineNumber)
		lineNumber += 1
		require.Equal(t, art.size, uint64(len(lines)-lineNumber+1))
	}
}

func TestLongestPrefix(t *testing.T) {
	r := NewRadixTree[any]()

	keys := []string{
		"",
		"foo",
		"foobar",
		"foobarbaz",
		"foobarbazzip",
		"foozip",
	}
	for _, k := range keys {
		r.Insert([]byte(k), nil)
	}
	if int(r.size) != len(keys) {
		t.Fatalf("bad len: %v %v", r.size, len(keys))
	}

	type exp struct {
		inp string
		out string
	}
	cases := []exp{
		{"a", ""},
		{"abc", ""},
		{"fo", ""},
		{"foo", "foo"},
		{"foob", "foo"},
		{"foobar", "foobar"},
		{"foobarba", "foobar"},
		{"foobarbaz", "foobarbaz"},
		{"foobarbazzi", "foobarbaz"},
		{"foobarbazzip", "foobarbazzip"},
		{"foozi", "foo"},
		{"foozip", "foozip"},
		{"foozipzap", "foozip"},
	}
	for _, test := range cases {
		m, _, ok := r.LongestPrefix([]byte(test.inp))
		if !ok {
			t.Fatalf("no match: %v", test)
		}
		if string(m) != test.out {
			t.Fatalf("mis-match: %v %v", string(m), test)
		}
	}
}

const datasetSize = 100000

func generateDataset(size int) []string {
	rand.Seed(time.Now().UnixNano())
	dataset := make([]string, size)
	for i := 0; i < size; i++ {
		uuid1, _ := uuid.GenerateUUID()
		dataset[i] = uuid1
	}
	return dataset
}

func BenchmarkMixedOperations(b *testing.B) {
	dataset := generateDataset(datasetSize)
	art := NewRadixTree[int]()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for j := 0; j < datasetSize; j++ {
			key := dataset[j]

			// Randomly choose an operation
			switch rand.Intn(3) {
			case 0:
				art.Insert([]byte(key), j)
			case 1:
				art.Get([]byte(key))
			case 2:
				art.Delete([]byte(key))
			}
		}
	}
}

func BenchmarkInsertART(b *testing.B) {
	r := NewRadixTree[int]()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		uuid1, _ := uuid.GenerateUUID()
		r.Insert([]byte(uuid1), n)
	}
}

func BenchmarkSearchART(b *testing.B) {
	r := NewRadixTree[int]()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		uuid1, _ := uuid.GenerateUUID()
		r.Insert([]byte(uuid1), n)
		r.Get([]byte(uuid1))
	}
}

func BenchmarkDeleteART(b *testing.B) {
	r := NewRadixTree[int]()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		uuid1, _ := uuid.GenerateUUID()
		r.Insert([]byte(uuid1), n)
		r.Delete([]byte(uuid1))
	}
}
