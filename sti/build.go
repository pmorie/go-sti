package sti

import (
	"bufio"
	"bytes"
	"github.com/fsouza/go-dockerclient"
	"io"
	"log"
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
	Writer      io.Writer
}

type BuildResult struct {
	Success  bool
	Messages []string
}

func Build(req BuildRequest) (*BuildResult, error) {
	c, err := newHandler(req.Request)
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

	if c.debug {
		if incremental {
			log.Printf("Existing image for tag %s detected for incremental build\n", req.Tag)
		} else {
			log.Printf("Clean build will be performed")
		}
	}

	var result *BuildResult

	if req.RuntimeImage == "" {
		result, err = c.build(req, incremental)
	} else {
		result, err = c.extendedBuild(req, incremental)
	}

	return result, err
}

func (c requestHandler) detectIncrementalBuild(tag string) (bool, error) {
	container, err := c.containerFromImage(tag)
	if err != nil {
		return false, err
	}
	defer c.dockerClient.RemoveContainer(docker.RemoveContainerOptions{container.ID, true})

	return FileExistsInContainer(c.dockerClient, container.ID, "/usr/bin/save-artifacts"), nil
}

func (c requestHandler) build(req BuildRequest, incremental bool) (*BuildResult, error) {
	if c.debug {
		log.Printf("Performing source build from %s\n", req.Source)
	}
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
	err := c.prepareSourceDir(req.Source, targetSourceDir)
	if err != nil {
		return nil, err
	}

	return c.buildDeployableImage(req, req.WorkingDir, incremental)
}

func (c requestHandler) extendedBuild(req BuildRequest, incremental bool) (*BuildResult, error) {
	return nil, nil
}

func (c requestHandler) saveArtifacts(image string, path string) error {
	if c.debug {
		log.Printf("Saving build artifacts from image %s to path %s\n", image, path)
	}

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

func (c requestHandler) prepareSourceDir(source string, targetSourceDir string) error {
	re := regexp.MustCompile("^git://")

	if re.MatchString(source) {
		if c.debug {
			log.Printf("Fetching %s to directory %s", source, targetSourceDir)
		}
		err := gitClone(source, targetSourceDir)
		if err != nil {
			return err
		}
	} else {
		// TODO: investigate using bind-mounts instead
		copy(source, targetSourceDir)
	}

	return nil
}

var dockerFileTemplate = template.Must(template.New("Dockerfile").Parse("" +
	"FROM {{.BaseImage}}\n" +
	"ADD ./src /usr/src\n" +
	"{{if .Incremental}}ADD ./artifacts /usr/artifacts\n{{end}}" +
	"{{range .Environment}}ENV {{.Name}} {{.Value}}\n{{end}}" +
	"RUN /usr/bin/prepare\n" +
	"CMD /usr/bin/run\n"))

func (c requestHandler) buildDeployableImage(req BuildRequest, contextDir string, incremental bool) (*BuildResult, error) {
	dockerFilePath := filepath.Join(contextDir, "Dockerfile")
	dockerFile, err := openFileExclusive(dockerFilePath, 0700)
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

	if c.debug {
		log.Printf("Wrote Dockerfile for build to %s\n", dockerFilePath)
	}

	tarBall, err := tarDirectory(contextDir)
	if err != nil {
		return nil, err
	}

	if c.debug {
		log.Printf("Created tarball for %s at %s\n", contextDir, tarBall.Name())
	}

	tarInput, err := os.Open(tarBall.Name())
	if err != nil {
		return nil, err
	}
	defer tarInput.Close()
	tarReader := bufio.NewReader(tarInput)
	var output []string

	if req.Writer != nil {
		err = c.dockerClient.BuildImage(docker.BuildImageOptions{req.Tag, false, false, true, tarReader, req.Writer, ""})
	} else {
		var buf []byte
		writer := bytes.NewBuffer(buf)
		err = c.dockerClient.BuildImage(docker.BuildImageOptions{req.Tag, false, false, true, tarReader, writer, ""})
		rawOutput := writer.String()
		output = strings.Split(rawOutput, "\n")
	}

	if err != nil {
		return nil, err
	}

	return &BuildResult{true, output}, nil
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
