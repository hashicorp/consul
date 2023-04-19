package test

import (
	"archive/zip"
	"io"
	"os"
)

func unzip(zipPath, destPath string) error {
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	zf := zr.File[0]
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
