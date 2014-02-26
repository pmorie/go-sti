package sti

import (
	"bytes"
	"github.com/fsouza/go-dockerclient"
	"log"
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

type DockerConnection struct {
	dockerClient *docker.Client
	debug        bool
}

func newConnection(req *Request) (*DockerConnection, error) {
	dockerClient, err := docker.NewClient(req.DockerSocket)

	if err != nil {
		return nil, ErrDockerConnectionFailed
	}

	return &DockerConnection{dockerClient, req.Debug}, nil
}

func (c DockerConnection) isImageInLocalRegistry(imageName string) (bool, error) {
	image, err := c.dockerClient.InspectImage(imageName)

	if image != nil {
		return true, nil
	} else if err == docker.ErrNoSuchImage {
		return false, nil
	}

	return false, err
}

func (c DockerConnection) containerFromImage(imageName string) (*docker.Container, error) {
	// TODO: set command?
	config := docker.Config{Image: imageName, AttachStdout: false, AttachStderr: false, Cmd: []string{"/bin/true"}}
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

	if exitCode != 0 {
		log.Printf("Container exit code: %d\n", exitCode)
		return nil, ErrCreateContainerFailed
	}

	return container, nil
}

func (c DockerConnection) checkAndPull(imageName string) (*docker.Image, error) {
	image, err := c.dockerClient.InspectImage(imageName)
	if err != nil {
		return nil, ErrPullImageFailed
	}

	if image == nil {
		if c.debug {
			log.Printf("Pulling image %s\n", imageName)
		}

		err = c.dockerClient.PullImage(docker.PullImageOptions{Repository: imageName}, docker.AuthConfiguration{})
		if err != nil {
			return nil, ErrPullImageFailed
		}

		image, err = c.dockerClient.InspectImage(imageName)
		if err != nil {
			return nil, err
		}
	} else if c.debug {
		log.Printf("Image %s available locally\n", imageName)
	}

	return image, nil
}

func (c DockerConnection) hasEntryPoint(image *docker.Image) bool {
	return image.Config.Entrypoint != nil
}

func (c DockerConnection) fileExistsInContainer(cId string, path string) bool {
	var buf []byte
	writer := bytes.NewBuffer(buf)

	err := c.dockerClient.CopyFromContainer(docker.CopyFromContainerOptions{writer, cId, path})
	content := writer.String()

	if c.debug {
		log.Printf("File %s in container %s: {%s}\n", path, cId, content)
	}

	return ((err == nil) && ("" != content))
}
