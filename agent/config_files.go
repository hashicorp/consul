package agent

import (
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
)

type fileInfo struct {
	// path is the full path to the file.
	path string

	// b is the content of the file. nil if there was an error.
	b []byte

	// err is set if the file could not be read.
	err error
}

// readPaths reads the contents of the paths according to the rules of
// readPath.
func readPaths(paths, exts []string, level int) []fileInfo {
	var fis []fileInfo
	for _, path := range paths {
		fis = append(fis, readPath(path, exts, level)...)
	}
	return fis
}

// readPath reads the contents of all files with extensions listed in
// exts recursively up to the given level starting from root. Files and
// directories are processed in lexicographical order. The function
// returns a fileInfo struct for every file that was not skipped.
func readPath(root string, exts []string, level int) []fileInfo {
	if level <= 0 {
		return nil
	}

	var fis []fileInfo
	err := filepath.Walk(root, func(fpath string, info os.FileInfo, err error) error {
		// fmt.Println("walk: path=", fpath)
		if err != nil {
			return err
		}

		if info.IsDir() {
			fis = append(fis, readPath(fpath, exts, level-1)...)
			return nil
		}

		// skip non-config file
		if !strSliceContains(exts, path.Ext(fpath)) {
			return nil
		}

		// read config file
		b, err := ioutil.ReadFile(fpath)
		if err != nil {
			fis = append(fis, fileInfo{path: fpath, err: err})
			return nil
		}
		fis = append(fis, fileInfo{path: fpath, b: b})
		return nil
	})
	if err != nil {
		fis = append(fis, fileInfo{path: root, err: err})
	}
	return fis
}

func strSliceContains(s []string, val string) bool {
	for _, v := range s {
		if v == val {
			return true
		}
	}
	return false
}
