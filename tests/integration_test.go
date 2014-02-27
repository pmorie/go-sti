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

// Register gocheck with the 'testing' runner
func Test(t *testing.T) { TestingT(t) }

type IntegrationTestSuite struct {
	dockerClient *docker.Client
	tempDir      string
}

// Register IntegrationTestSuite with the gocheck suite manager
var _ = Suite(&IntegrationTestSuite{})

const (
	DockerSocket        = "unix:///var/run/docker.sock"
	TestSource          = "git://github.com/pmorie/simple-html"
	FakeBaseImage       = "pmorie/sti-fake"
	BrokenBaseImage     = "pmorie/sti-fake-broken"
	TagCleanBuild       = "sti/test-fake-app"
	TagIncrementalBuild = "sti/test-incremental-app"
)

// Add support for 'go test' flags, viz: go test -integration -extended
var integration = flag.Bool("integration", false, "Include integration tests")
var extended = flag.Bool("extended", false, "Include long-running tests")

// Suite/Test fixtures are provided by gocheck
func (s *IntegrationTestSuite) SetUpSuite(c *C) {
	if !*integration {
		c.Skip("-integration not provided")
	}

	s.dockerClient, _ = docker.NewClient(DockerSocket)
	s.tempDir, _ = ioutil.TempDir("", "go-sti-integration")
}

func (s *IntegrationTestSuite) SetUpTest(c *C) {
	s.dockerClient.RemoveImage("sti/test-fake-app")
}

// TestXxxx methods are identified as test cases
func (s *IntegrationTestSuite) TestValidateSuccess(c *C) {
	req := sti.ValidateRequest{
		Request: &sti.Request{
			Configuration: sti.Configuration{
				WorkingDir:   s.tempDir,
				DockerSocket: DockerSocket,
				Debug:        true},
			BaseImage: FakeBaseImage,
		},
		Incremental: false,
	}
	resp, err := sti.Validate(req)
	c.Assert(err, IsNil, Commentf("Validation failed: err"))
	c.Assert(resp.Valid, Equals, true, Commentf("Validation failed: invalid response"))
}

func (s *IntegrationTestSuite) TestValidateFailure(c *C) {
	req := sti.ValidateRequest{
		Request: &sti.Request{
			Configuration: sti.Configuration{
				WorkingDir:   s.tempDir,
				DockerSocket: DockerSocket,
				Debug:        true},
			BaseImage: BrokenBaseImage,
		},
		Incremental: false,
	}
	resp, err := sti.Validate(req)
	c.Assert(err, IsNil, Commentf("Validation failed: err"))
	c.Assert(resp.Valid, Equals, false, Commentf("Validation should have failed: invalid response"))
}

func (s *IntegrationTestSuite) TestValidateIncrementalSuccess(c *C) {
	req := sti.ValidateRequest{
		Request: &sti.Request{
			Configuration: sti.Configuration{
				WorkingDir:   s.tempDir,
				DockerSocket: DockerSocket,
				Debug:        true},
			BaseImage:    FakeBaseImage,
			RuntimeImage: FakeBaseImage,
		},
	}
	resp, err := sti.Validate(req)
	c.Assert(err, IsNil, Commentf("Validation failed: err"))
	c.Assert(resp.Valid, Equals, true, Commentf("Validation failed: invalid response"))
}

// Test a clean build.  The simplest case.
func (s *IntegrationTestSuite) TestCleanBuild(c *C) {
	tag := TagCleanBuild
	req := sti.BuildRequest{
		Request: &sti.Request{
			Configuration: sti.Configuration{
				WorkingDir:   s.tempDir,
				DockerSocket: DockerSocket,
				Debug:        true},
			BaseImage: FakeBaseImage},
		Clean:  true,
		Source: TestSource,
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

// Test an incremental build.
func (s *IntegrationTestSuite) TestIncrementalBuild(c *C) {
	if !*extended {
		c.Skip("-extended not provided")
	}

	tag := TagIncrementalBuild
	req := sti.BuildRequest{
		Request: &sti.Request{
			Configuration: sti.Configuration{
				WorkingDir:   s.tempDir,
				DockerSocket: DockerSocket,
				Debug:        true},
			BaseImage: FakeBaseImage},
		Clean:  true,
		Source: TestSource,
		Tag:    tag,
		Writer: os.Stdout}
	resp, err := sti.Build(req)
	c.Assert(err, IsNil, Commentf("Sti build failed"))
	c.Assert(resp.Success, Equals, true, Commentf("Sti build failed"))

	req.Clean = false
	resp, err = sti.Build(req)
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
	res := sti.FileExistsInContainer(s.dockerClient, cId, filePath)

	c.Assert(res, Equals, true, Commentf("Couldn't find file %s in container %s", filePath, cId))
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
