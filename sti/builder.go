package sti

import (
	"archive/tar"
	"bufio"
	"bytes"
	"fmt"
	"github.com/fsouza/go-dockerclient"
	"os"
	"path/filepath"
)

type Env struct {
	Name  string
	Value string
}

type BuildRequest struct {
	Request
	Source      string
	Tag         string
	Clean       bool
	Environment []Env
}

type BuildResult struct {
	Success bool
	Output  []string
}

func Build(req BuildRequest) (BuildResult, error) {
	fmt.Printf("%+v\n", req)

	c, err := newConnection(req)

	if err != nil {
		return nil, err
	}

	incremental := !req.Clean

	if incremental {
		exists, err = c.isImageInLocalRegistry(req.Tag)

		if err != nil {
			return nil, err
		}

		if exists {
			incremental, err = detectIncrementalBuild(req.Tag)
			if err != nil {
				return nil, err
			}
		}
	}

	var result BuildResult

	if req.RuntimeImage != "" {
		result, err = c.build(req, incremental)
	} else {
		result, err = c.extendedBuild(req, incremental)
	}

	return result, err
}

func (c DockerConnection) detectIncrementalBuild(tag string) (bool, error) {
	container, err := c.containerFromImage(tag)
	if err != nil {
		return nil, err
	}
	defer c.dockerClient.RemoveContainer(docker.RemoveContainersOptions{container.ID, true})

	return c.fileExistsInContainer(container.ID, "/usr/bin/save-artifacts")
}

func (c DockerConnection) saveArtifacts(image string, path string) error {
	container, err := c.dockerClient.CreateContainer(docker.CreateContainerOptions{})
}

func (c DockerConnection) build(req BuildRequest, incremental bool) (BuildResult, error) {
	if incremental {
		artifactTmpDir := filepath.Join(req.WorkingDir, "artifacts")
		err := os.Mkdir(artifactTmpDir, 0700)

		if err != nil {
			return nil, err
		}
		c.saveArtifacts(req.Tag, artifactTmpDir)
	}

	return nil, nil
}

func (c DockerConnection) extendedBuild(req BuildRequest, incremental bool) (BuildResult, error) {
	return nil, nil
}

var dockerFileTemplate = template.Must(template.New("Dockerfile").Parse("" +
	"FROM {{.BaseImage}}\n" +
	"ADD ./src /usr/src\n" +
	"{{ if .Incremental }}\n" +
	"ADD ./artifacts /usr/artifacts\n" +
	// TODO env
	"RUN /usr/bin/prepare\n" +
	"CMD /usr/bin/run\n"))

func (c DockerConnection) buildDeployableImage(req BuildRequest, contextDir string, incremental bool) (BuildResult, err) {
	dockerFile, err := openFileExclusive(filepath.Join(contextDir, "Dockerfile"), 0700)

	if err != nil {
		return nil, err
	}

	templateFiller := struct {
		BaseImage   string
		Incremental bool
	}{req.BaseImage, incremental}

	err = dockerFileTemplate.Execute(dockerFile, templateFiller)
	dockerFile.Close()
	if err != nil {
		return nil, CreateDockerfileFailed
	}

	tarBall := tarDirectory(contextDir)
	tarReader := bufio.NewReader(tarBall)
	// defer tarReader.Close() // TODO: necessary

	var buf bytes.Buffer
	writer := bytes.NewBuffer(buf)

	err = c.dockerClient.BuildImage(docker.BuildImageOptions{req.Tag, false, false, true, tarReader, writer, nil})

	rawOutput := writer.String()
	output := rawOutput.Split("\n")
	// defer output.Close() // TODO: necessary

	return BuildResult{(err != nil), output}, nil
}

func openFileExclusive(path string, mode os.FileMode) (*os.File, error) {
	file, errf := os.OpenFile(path, os.O_CREATE|os.O_RDWR, mode)
	if errf != nil {
		return nil, errf
	}

	if errl := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); errl != nil {
		if errl == syscall.EWOULDBLOCK {
			return nil, CreateDockerfileFailed
		}
		return nil, errl
	}
}
