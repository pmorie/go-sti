package sti

import (
	"log"

	"github.com/fsouza/go-dockerclient"
)

// Request contains essential fields for any request: a Configuration, a base image, and an
// optional runtime image.
type Request struct {
	BaseImage    string
	RuntimeImage string

	DockerSocket  string
	DockerTimeout int
	WorkingDir    string
	Debug         bool
}

// requestHandler encapsulates dependencies needed to fulfill requests.
type requestHandler struct {
	dockerClient *docker.Client
	debug        bool
}

func newHandler(req Request) (*requestHandler, error) {
	log.Printf("Using docker socket: %s\n", req.DockerSocket)
	dockerClient, err := docker.NewClient(req.DockerSocket)

	if err != nil {
		return nil, ErrDockerConnectionFailed
	}

	return &requestHandler{dockerClient, req.Debug}, nil
}

func (h requestHandler) isImageInLocalRegistry(imageName string) (bool, error) {
	image, err := h.dockerClient.InspectImage(imageName)

	if image != nil {
		return true, nil
	} else if err == docker.ErrNoSuchImage {
		return false, nil
	}

	return false, err
}

func (h requestHandler) containerFromImage(imageName string) (*docker.Container, error) {
	config := docker.Config{Image: imageName, AttachStdout: false, AttachStderr: false, Cmd: []string{"/bin/true"}}
	container, err := h.dockerClient.CreateContainer(docker.CreateContainerOptions{Name: "", Config: &config})
	if err != nil {
		return nil, err
	}

	err = h.dockerClient.StartContainer(container.ID, &docker.HostConfig{})
	if err != nil {
		return nil, err
	}

	exitCode, err := h.dockerClient.WaitContainer(container.ID)
	if err != nil {
		return nil, err
	}

	if exitCode != 0 {
		log.Printf("Container exit code: %d\n", exitCode)
		return nil, ErrCreateContainerFailed
	}

	return container, nil
}

func (h requestHandler) checkAndPull(imageName string) (*docker.Image, error) {
	image, err := h.dockerClient.InspectImage(imageName)
	if err != nil {
		return nil, ErrPullImageFailed
	}

	if image == nil {
		if h.debug {
			log.Printf("Pulling image %s\n", imageName)
		}

		err = h.dockerClient.PullImage(docker.PullImageOptions{Repository: imageName}, docker.AuthConfiguration{})
		if err != nil {
			return nil, ErrPullImageFailed
		}

		image, err = h.dockerClient.InspectImage(imageName)
		if err != nil {
			return nil, err
		}
	} else if h.debug {
		log.Printf("Image %s available locally\n", imageName)
	}

	return image, nil
}
