package sti

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
}

func newConnection(req Request) (DockerConnection, error) {
	dockerClient, err := docker.NewClient(req.DockerSocket)

	if err != nil {
		return nil, DockerConnectionFailure
	}

	return DockerConnection{dockerClient}, nil
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

func (c DockerConnection) fileExistsInContainer(cId string, path string) bool {
	return nil == c.dockerClient.CopyFromContainer(docker.CopyFromContainerOptions{ioutil.Discard, cId, path})
}
