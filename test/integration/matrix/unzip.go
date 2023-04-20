package test

import (
	"archive/zip"
	"io"
	"os"
)

// Unzip the file at zipPath to destPath
// Only supports zip files with single binary entry (ie. our releases)
func unzip(zipPath, destPath string) error {
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	zf := zr.File[0] // release zip archive's single file
	zfp, err := zf.Open()
	if err != nil {
		return err
	}

	fp, err := os.OpenFile(destPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0755)
	if err != nil {
		return err
	}
	if _, err := io.Copy(fp, zfp); err != nil {
		return err
	}
	fp.Close()
	zr.Close()
	return nil
}
