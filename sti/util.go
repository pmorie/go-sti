package sti

import (
	"archive/tar"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func writeTar(path string, tw *tar.Writer, fi os.FileInfo) error {
	fr, err := os.Open(path)
	if err != nil {
		return err
	}
	defer fr.Close()

	h := new(tar.Header)
	h.Name = path
	h.Size = fi.Size()
	h.Mode = int64(fi.Mode())
	h.ModTime = fi.ModTime()

	err = tw.WriteHeader(h)
	if err != nil {
		return err
	}

	_, err = io.Copy(tw, fr)
	return err
}

func addDirectory(dirPath string, tw *tar.Writer) error {
	dir, err := os.Open(dirPath)
	if err != nil {
		return err
	}
	defer dir.Close()

	files, err := dir.Readdir(0)
	if err != nil {
		return err
	}

	for _, fi := range files {
		curPath := filepath.Join(dirPath, fi.Name())

		if fi.IsDir() {
			err = addDirectory(curPath, tw)
			if err != nil {
				return err
			}
		} else {
			err = writeTar(curPath, tw, fi)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func tarDirectory(dir string) (os.File, error) {
	fw, err := ioutil.TempDir("", "sti-tar")
	if err != nil {
		return nil, err
	}
	defer fw.Close()

	tw := tar.NewWriter(fw)
	defer tw.Close()

	addDirectory(dir, tw)

	return fw, nil
}
