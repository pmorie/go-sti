package tests

import (
	"flag"
	"github.com/fsouza/go-dockerclient"
	"github.com/pmorie/go-sti/sti"
	"io/ioutil"
	. "launchpad.net/gocheck"
	"os"
	"testing"
)

const dockerSocket = "unix:///var/run/docker.sock"

// Register gocheck with the 'testing' runner
func Test(t *testing.T) { TestingT(t) }

type IntegrationTestSuite struct {
	dockerClient *docker.Client
	tempDir      string
}

var integration = flag.Bool("integration", false, "Include integration tests")

// Register IntegrationTestSuite with the gocheck suite manager
var _ = Suite(&IntegrationTestSuite{})

// Suite/Test fixtures are provided by gocheck
func (s *IntegrationTestSuite) SetUpSuite(c *C) {
	if !*integration {
		c.Skip("-integration not provided")
	}

	s.dockerClient, _ = docker.NewClient(dockerSocket)
	s.tempDir, _ = ioutil.TempDir("", "go-sti-integration")
}

func (s *IntegrationTestSuite) SetUpTest(c *C) {
	s.dockerClient.RemoveImage("sti/test-fake-app")
}

// TestXxxx methods are identified as test cases
func (s *IntegrationTestSuite) TestCleanBuild(c *C) {
	tag := "sti/test-fake-app"
	gitRepo := "git://github.com/pmorie/simple-html"
	baseImage := "pmorie/sti-fake"

	req := sti.BuildRequest{
		Request: &sti.Request{
			Configuration: sti.Configuration{
				WorkingDir:   s.tempDir,
				DockerSocket: dockerSocket,
				Debug:        true},
			BaseImage: baseImage},
		Clean:  true,
		Source: gitRepo,
		Tag:    tag,
		Writer: os.Stdout}
	resp, err := sti.Build(req)
	c.Assert(err, IsNil, Commentf("Sti build failed"))
	c.Assert(resp.Success, Equals, true, Commentf("Sti build failed"))

	s.checkForImage(c, tag)

	containerId := s.createContainer(c, tag)
	defer s.removeContainer(containerId)
	s.checkBasicBuildState(c, containerId)
}

func (s *IntegrationTestSuite) checkForImage(c *C, tag string) {
	_, err := s.dockerClient.InspectImage(tag)
	c.Assert(err, IsNil, Commentf("Couldn't find built image"))
}

func (s *IntegrationTestSuite) createContainer(c *C, image string) string {
	config := docker.Config{Image: image, AttachStdout: false, AttachStdin: false}
	container, err := s.dockerClient.CreateContainer(docker.CreateContainerOptions{Name: "", Config: &config})
	c.Assert(err, IsNil, Commentf("Couldn't create container from image %s", image))

	err = s.dockerClient.StartContainer(container.ID, &docker.HostConfig{})
	c.Assert(err, IsNil, Commentf("Couldn't start container: %s", container.ID))

	exitCode, err := s.dockerClient.WaitContainer(container.ID)
	c.Assert(exitCode, Equals, 0, Commentf("Bad exit code from container: %d", exitCode))

	return container.ID
}

func (s *IntegrationTestSuite) removeContainer(cId string) {
	s.dockerClient.RemoveContainer(docker.RemoveContainerOptions{cId, true})
}

func (s *IntegrationTestSuite) checkFileExists(c *C, cId string, filePath string) {
	err := s.dockerClient.CopyFromContainer(docker.CopyFromContainerOptions{ioutil.Discard, cId, filePath})

	c.Assert(err, IsNil, Commentf("Couldn't find file %s in container %s", filePath, cId))
}

func (s *IntegrationTestSuite) checkBasicBuildState(c *C, cId string) {
	s.checkFileExists(c, cId, "/sti-fake/prepare-invoked")
	s.checkFileExists(c, cId, "/sti-fake/run-invoked")
	s.checkFileExists(c, cId, "/sti-fake/src/index.html")
}

func (s *IntegrationTestSuite) checkIncrementalBuildState(c *C, cId string) {
	s.checkBasicBuildState(c, cId)
	s.checkFileExists(c, cId, "/sti-fake/save-artifacts-invoked")
}

func (s *IntegrationTestSuite) checkExtendedBuildState(c *C, cId string) {
	s.checkFileExists(c, cId, "/sti-fake/prepare-invoked")
	s.checkFileExists(c, cId, "/sti-fake/run-invoked")
}

func (s *IntegrationTestSuite) checkIncrementalExtendedBuildState(c *C, cId string) {
	s.checkExtendedBuildState(c, cId)
	s.checkFileExists(c, cId, "/sti-fake/src/save-artifacts-invoked")
}
