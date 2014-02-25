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

type Configuration struct {
	DockerSocket  string
	DockerTimeout int
	WorkingDir    string
	Debug         bool
}

type Request struct {
	Configuration
	BaseImage    string
	RuntimeImage string
}

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

type ValidateRequest struct {
	Request
	Incremental bool
}

type ValidateResult struct {
	Valid  bool
	Errors []string
}

func Build(req BuildRequest) (BuildResult, error) {
	fmt.Printf("%+v\n", req)

	connection, err := newConnection(req)

	if err != nil {
		return nil, err
	}

	incremental := req.Clean

	if incremental {
		incremental, err = detectIncrementalBuild(req.Tag)

		if err != nil {
			return nil, err
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

func (c DockerConnection) build(req BuildRequest, incremental bool) (BuildResult, error) {
	if incremental {
		// TODO: make temp dir for binding into build container
	}

	return nil, nil
}

func (c DockerConnection) extendedBuild(req BuildRequest, incremental bool) (BuildResult, error) {
	return nil, nil
}

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

	tarBall = tarDirectory(contextDir)
	tarReader = bufio.NewReader(tarBall)
	// TODO: create tarfile of context
	// TODO: input stream for context
	// TODO: output stream for results
	var buf bytes.Buffer
	err = c.dockerClient.BuildImage(docker.BuildImageOptions{req.Tag, false, false, true, nil, nil, nil})

	if err != nil {
		return BuildResult{Success: false}, nil
	}

	return nil, nil
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

var dockerFileTemplate = template.Must(template.New("Dockerfile").Parse("" +
	"FROM {{.BaseImage}}\n" +
	"ADD ./src /usr/src\n" +
	"{{ if .Incremental }}\n" +
	"ADD ./artifacts /usr/artifacts\n" +
	// TODO env
	"RUN /usr/bin/prepare\n" +
	"CMD /usr/bin/run\n"))

func Validate(req ValidateRequest) (ValidateResult, error) {
	connection, err := newConnection(req)
	result := ValidateResult{Valid: true}

	if err != nil {
		return nil, err
	}

	if req.RuntimeImage != "" {
		valid, err := c.validateImage(req.BaseImage, false)

		if err != nil {
			return nil, err
		}

		&result.recordValidation("Base image", req.BaseImage, valid)

		valid, err = c.validateImage(req.RuntimeImage, true)

		if err != nil {
			return nil, err
		}

		&result.recordValidation("Runtime image", req.RuntimeImage, valid)
	} else {
		valid, err := c.validateImage(req.BaseImage, req.Incremental)

		if err != nil {
			return nil, err
		}

		&result.recordValidation("Base image", req.BaseImage, valid)
	}

	return result, nil
}

func (res *ValidationResult) recordValidation(what string, image string, valid bool) {
	if !valid {
		res.Valid = false
		res.Errors = append(res.Errors, fmt.Sprintf("%s %s failed validation", what, image))
	}
}

type DockerConnection struct {
	dockerClient *docker.Client
}

func newConnection(req Request) (DockerConnection, error) {
	dockerClient, err := docker.NewClient(req.DockerSocket)

	if err != nil {
		return nil, DockerConnectionFailure
	}

	return DockerConnection{dockerClient}, nil
}

func (c DockerConnection) validateImage(imageName string, incremental bool) (bool, error) {
	image, err := c.checkAndPull(imageName)

	if err != nil {
		return false, err
	}

	if c.hasEntryPoint(image) {
		return false, nil
	}

	files := []string{"/usr/bin/prepare", "/usr/bin/run"}

	if incremental {
		files = append(files, "/usr/bin/save-artifacts")
	}

	valid, err := c.validateRequiredFiles(imageName, files)

	if err != nil {
		return nil, err
	}

	return valid, nil
}

func (c DockerConnection) containerFromImage(imageName string) (docker.Container, error) {
	// TODO: set command?
	config := docker.Config{Image: imageName, AttachStdout: false, AttachStdout: false}
	container, err := c.dockerClient.CreateContainer(docker.CreateContainerOptions{Name: "", Config: &config})

	if err != nil {
		return nil, err
	}

	err = c.dockerClient.StartContainer(container.ID, &docker.HostConfig{})

	if err != nil {
		return nil, err
	}

	exitCode, err := c.dockerClient.WaitContainer(container.ID)

	if err != nil {
		return nil, err
	}

	return container, nil
}

func (c DockerConnection) checkAndPull(imageName string) (*docker.Image, error) {
	image, err := c.dockerClient.InspectImage(imageName)

	if err != nil {
		return nil, PullImageFailed
	}

	if image == nil {
		image, err = c.dockerClient.PullImage(imageName)

		if err != nil {
			return nil, PullImageFailed
		}
	}

	return image, nil
}

func (c DockerConnection) hasEntryPoint(image *docker.Image) bool {
	return image.Config.Entrypoint != nil
}

func (c DockerConnection) validateRequiredFiles(imageName string, files []string) (bool, error) {
	container, err = c.containerFromImage(imageName)
	if err != nil {
		return false, CreateContainerFailed
	}
	defer c.dockerClient.RemoveContainer(docker.RemoveContainersOptions{container.ID, true})

	for _, file := range files {
		if !c.fileExistsInContainer(container.ID, file) {
			return false, nil
		}
	}

	return true, nil
}

func (c DockerConnection) fileExistsInContainer(cId string, path string) bool {
	return nil == c.dockerClient.CopyFromContainer(docker.CopyFromContainerOptions{ioutil.Discard, cId, path})
}
