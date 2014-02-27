package sti

import (
	"archive/tar"
	"bytes"
	"github.com/fsouza/go-dockerclient"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Determine whether a file exists in a container.
func FileExistsInContainer(dockerClient *docker.Client, cId string, path string) bool {
	var buf []byte
	writer := bytes.NewBuffer(buf)

	err := dockerClient.CopyFromContainer(docker.CopyFromContainerOptions{writer, cId, path})
	content := writer.String()

	return ((err == nil) && ("" != content))
}

func writeTar(tw *tar.Writer, path string, relative string, fi os.FileInfo) error {
	fr, err := os.Open(path)
	if err != nil {
		return err
	}
	defer fr.Close()

	h := new(tar.Header)
	h.Name = strings.Replace(path, relative, ".", 1)
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

func tarDirectory(dir string) (*os.File, error) {
	fw, err := ioutil.TempFile("", "sti-tar")
	if err != nil {
		return nil, err
	}
	defer fw.Close()

	tw := tar.NewWriter(fw)
	defer tw.Close()

	err = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if !info.IsDir() {
			err = writeTar(tw, path, dir, info)
			if err != nil {
				return err
			}
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return fw, nil
}

func copy(sourcePath string, targetPath string) error {
	cmd := exec.Command("cp", "-ad", sourcePath, targetPath)
	return cmd.Run()
}

func gitClone(source string, targetPath string) error {
	cmd := exec.Command("git", "clone", "--quiet", source, targetPath)
	err := cmd.Run()

	return err
}

func imageHasEntryPoint(image *docker.Image) bool {
	return image.Config.Entrypoint != nil
}

func openFileExclusive(path string, mode os.FileMode) (*os.File, error) {
	file, errf := os.OpenFile(path, os.O_CREATE|os.O_RDWR, mode)
	if errf != nil {
		return nil, errf
	}

	if errl := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); errl != nil {
		if errl == syscall.EWOULDBLOCK {
			return nil, ErrCreateDockerfileFailed
		}

		return nil, errl
	}

	return file, nil
}
