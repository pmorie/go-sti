package sti

import (
	"fmt"
	"github.com/fsouza/go-dockerclient"
)

type ValidateRequest struct {
	*Request
	Incremental bool
}

type ValidateResult struct {
	Valid  bool
	Errors []string
}

func Validate(req ValidateRequest) (*ValidateResult, error) {
	c, err := newConnection(req.Request)
	result := &ValidateResult{Valid: true}

	if err != nil {
		return nil, err
	}

	if req.RuntimeImage != "" {
		valid, err := c.validateImage(req.BaseImage, false)

		if err != nil {
			return nil, err
		}

		result.recordValidation("Base image", req.BaseImage, valid)

		valid, err = c.validateImage(req.RuntimeImage, true)

		if err != nil {
			return nil, err
		}

		result.recordValidation("Runtime image", req.RuntimeImage, valid)
	} else {
		valid, err := c.validateImage(req.BaseImage, req.Incremental)

		if err != nil {
			return nil, err
		}

		result.recordValidation("Base image", req.BaseImage, valid)
	}

	return result, nil
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
		return false, err
	}

	return valid, nil
}

func (c DockerConnection) validateRequiredFiles(imageName string, files []string) (bool, error) {
	container, err := c.containerFromImage(imageName)
	if err != nil {
		return false, ErrCreateContainerFailed
	}
	defer c.dockerClient.RemoveContainer(docker.RemoveContainerOptions{container.ID, true})

	for _, file := range files {
		if !c.fileExistsInContainer(container.ID, file) {
			return false, nil
		}
	}

	return true, nil
}

func (res *ValidateResult) recordValidation(what string, image string, valid bool) {
	if !valid {
		res.Valid = false
		res.Errors = append(res.Errors, fmt.Sprintf("%s %s failed validation", what, image))
	}
}
