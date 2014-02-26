package sti

import (
	"bufio"
	"bytes"
	"fmt"
	"github.com/fsouza/go-dockerclient"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"text/template"
)

type Env struct {
	Name  string
	Value string
}

type BuildRequest struct {
	*Request
	Source      string
	Tag         string
	Clean       bool
	Environment []Env
}

type BuildResult struct {
	Success bool
	Output  []string
}

func Build(req BuildRequest) (*BuildResult, error) {
	fmt.Printf("%+v\n", req)

	c, err := newConnection(req.Request)

	if err != nil {
		return nil, err
	}

	incremental := !req.Clean

	if incremental {
		exists, err := c.isImageInLocalRegistry(req.Tag)

		if err != nil {
			return nil, err
		}

		if exists {
			incremental, err = c.detectIncrementalBuild(req.Tag)
			if err != nil {
				return nil, err
			}
		}
	}

	var result *BuildResult

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
		return false, err
	}
	defer c.dockerClient.RemoveContainer(docker.RemoveContainerOptions{container.ID, true})

	return c.fileExistsInContainer(container.ID, "/usr/bin/save-artifacts"), nil
}

func (c DockerConnection) saveArtifacts(image string, path string) error {
	var volumeMap map[string]struct{}
	volumeMap = make(map[string]struct{})
	volumeMap["/usr/artifacts"] = *new(struct{})

	config := docker.Config{Image: image, Cmd: []string{"/usr/bin/save-artifacts"}, Volumes: volumeMap}
	container, err := c.dockerClient.CreateContainer(docker.CreateContainerOptions{Name: "", Config: &config})

	if err != nil {
		return err
	}

	hostConfig := docker.HostConfig{Binds: []string{path + ":/usr/artifacts"}}
	err = c.dockerClient.StartContainer(container.ID, &hostConfig)
	exitCode, err := c.dockerClient.WaitContainer(container.ID)

	if err != nil {
		return err
	}

	if exitCode != 0 {
		return ErrSaveArtifactsFailed
	}

	return nil
}

func (c DockerConnection) build(req BuildRequest, incremental bool) (*BuildResult, error) {
	if incremental {
		artifactTmpDir := filepath.Join(req.WorkingDir, "artifacts")
		err := os.Mkdir(artifactTmpDir, 0700)
		if err != nil {
			return nil, err
		}

		err = c.saveArtifacts(req.Tag, artifactTmpDir)
		if err != nil {
			return nil, err
		}
	}

	targetSourceDir := filepath.Join(req.WorkingDir, "src")
	c.prepareSourceDir(req.Source, targetSourceDir)

	return c.buildDeployableImage(req, req.WorkingDir, incremental)
}

func (c DockerConnection) extendedBuild(req BuildRequest, incremental bool) (*BuildResult, error) {
	return nil, nil
}

func (c DockerConnection) prepareSourceDir(source string, targetSourceDir string) {
	re := regexp.MustCompile("^git://")

	if re.MatchString(source) {
		// TODO: git clone
	} else {
		// TODO: investigate using bind-mounts instead
		copy(source, targetSourceDir)
	}
}

var dockerFileTemplate = template.Must(template.New("Dockerfile").Parse("" +
	"FROM {{.BaseImage}}\n" +
	"ADD ./src /usr/src\n" +
	"{{if .Incremental}}ADD ./artifacts /usr/artifacts\n{{end}}" +
	"{{range .Environment}}ENV {{.Name}} {{.Value}}\n{{end}}" +
	"RUN /usr/bin/prepare\n" +
	"CMD /usr/bin/run\n"))

func (c DockerConnection) buildDeployableImage(req BuildRequest, contextDir string, incremental bool) (*BuildResult, error) {
	dockerFile, err := openFileExclusive(filepath.Join(contextDir, "Dockerfile"), 0700)
	if err != nil {
		return nil, err
	}
	defer dockerFile.Close()

	templateFiller := struct {
		BaseImage   string
		Environment []Env
		Incremental bool
	}{req.BaseImage, req.Environment, incremental}
	err = dockerFileTemplate.Execute(dockerFile, templateFiller)
	if err != nil {
		return nil, ErrCreateDockerfileFailed
	}

	tarBall, err := tarDirectory(contextDir)
	if err != nil {
		return nil, err
	}

	tarReader := bufio.NewReader(tarBall)
	var buf []byte
	writer := bytes.NewBuffer(buf)

	err = c.dockerClient.BuildImage(docker.BuildImageOptions{req.Tag, false, false, true, tarReader, writer, ""})
	rawOutput := writer.String()
	output := strings.Split(rawOutput, "\n")

	return &BuildResult{(err != nil), output}, nil
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
