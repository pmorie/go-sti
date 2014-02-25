package sti

type StiError int

const (
	DockerConnectionFailure StiError = iota
	NoSuchBaseImage
	NoSuchRuntimeImage
	PullImageFailed
	CreateDockerfileFailed
	CreateContainerFailed
)
