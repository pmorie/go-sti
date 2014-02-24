package sti

import (
	"fmt"
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

type ValidateRequest struct {
	Request
	Incremental bool
}

func Build(req BuildRequest) error {
	fmt.Printf("%+v\n", req)
	return nil
}

func Validate(req ValidateRequest) error {
	fmt.Printf("%+v\n", req)
	return nil
}
