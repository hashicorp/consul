// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package adaptive

import (
	"bufio"
	"github.com/hashicorp/go-uuid"
	"os"
	"testing"

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
		lineNumberFetched, f := art.Search([]byte(line))
		require.True(t, f)
		require.Equal(t, lineNumberFetched, lineNumber)
		lineNumber += 1
	}

	artLeafMin := art.Minimum()
	artLeafMax := art.Maximum()
	require.Equal(t, artLeafMin.key, getTreeKey([]byte("A")))
	require.Equal(t, artLeafMax.key, getTreeKey([]byte("zythum")))
}

//func TestARTree_InsertAndSearchUUID(t *testing.T) {
//	t.Parallel()
//
//	art := NewRadixTree[int]()
//
//	file, err := os.Open("test-text/uuid.txt")
//	if err != nil {
//		t.Fatal(err)
//	}
//	defer file.Close()
//
//	var lines []string
//
//	scanner := bufio.NewScanner(file)
//	// optionally, resize scanner's capacity for lines over 64K, see next example
//	lineNumber := 1
//	for scanner.Scan() {
//		art.Insert(scanner.Bytes(), lineNumber)
//		lineNumber += 1
//		lines = append(lines, scanner.Text())
//	}
//
//	// optionally, resize scanner's capacity for lines over 64K, see next example
//	lineNumber = 1
//	for _, line := range lines {
//		lineNumberFetched, f := art.Search([]byte(line))
//		require.True(t, f)
//		require.Equal(t, lineNumberFetched, lineNumber)
//		lineNumber += 1
//	}
//
//	artLeafMin := art.Minimum()
//	require.Equal(t, artLeafMin.key, getTreeKey([]byte("00026bda-e0ea-4cda-8245-522764e9f325")))
//
//	artLeafMax := art.Maximum()
//	require.Equal(t, artLeafMax.key, getTreeKey([]byte("ffffcb46-a92e-4822-82af-a7190f9c1ec5")))
//}

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
		lineNumberFetched, f := art.Search([]byte(line))
		require.True(t, f)
		require.Equal(t, lineNumberFetched, lineNumber)
		val := art.Delete([]byte(line))
		require.Equal(t, val, lineNumber)
		lineNumber += 1
		require.Equal(t, art.size, uint64(len(lines)-lineNumber+1))
	}
}

func BenchmarkInsertSearchART(b *testing.B) {
	r := NewRadixTree[int]()
	maxV := 100000
	for i := 0; i < maxV; i++ {
		uuid1, _ := uuid.GenerateUUID()
		for j := 0; j < 10; j++ {
			uuidx, _ := uuid.GenerateUUID()
			uuid1 += uuidx
		}
		r.Insert([]byte(uuid1), i)
	}
	keys := make([]string, maxV)
	for i := 0; i < maxV; i++ {
		uuid1, _ := uuid.GenerateUUID()
		for j := 0; j < 10; j++ {
			uuidx, _ := uuid.GenerateUUID()
			uuid1 += uuidx
		}
		keys[i] = uuid1
	}
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		uuid1 := keys[n%maxV]
		r.Insert([]byte(uuid1), n)
		r.Search([]byte(uuid1))
		r.Delete([]byte(uuid1))
	}
}
