package cmd

import (
	"fmt"
	"github.com/pmorie/go-sti/sti"
	"github.com/smarterclayton/cobra"
	"io/ioutil"
	"os"
	"strings"
)

func parseEnvs(envStr string) ([]sti.Env, error) {
	// TODO: error handling
	var envs []sti.Env
	pairs := strings.Split(envStr, ",")

	for _, pair := range pairs {
		atoms := strings.Split(pair, "=")
		name := atoms[0]
		value := atoms[1]

		env := sti.Env{name, value}
		envs = append(envs, env)
	}

	return envs, nil
}

func Execute() {
	var (
		req       sti.Request
		envString string
	)

	buildReq := sti.BuildRequest{Request: &req}
	validateReq := sti.ValidateRequest{Request: &req}

	stiCmd := &cobra.Command{
		Use:   "sti",
		Short: "STI is a tool for building repeatable docker images",
		Long: `A command-line interface for the sti library
              Complete documentation is available at http://github.com/pmorie/go-sti`,
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Usage()
		},
	}
	stiCmd.PersistentFlags().StringVarP(&(req.DockerSocket), "url", "U", "unix:///var/run/docker.sock", "Set the url of the docker socket to use")
	stiCmd.PersistentFlags().IntVar(&(req.DockerTimeout), "timeout", 30, "Set the timeout for docker operations")

	buildCmd := &cobra.Command{
		Use:   "build SOURCE BASE_IMAGE TAG",
		Short: "Build an image",
		Long:  "Build an image",
		Run: func(cmd *cobra.Command, args []string) {
			buildReq.Source = args[0]
			buildReq.BaseImage = args[1]
			buildReq.Tag = args[2]

			envs, _ := parseEnvs(envString)
			buildReq.Environment = envs

			if buildReq.WorkingDir == "tempdir" {
				var err error
				buildReq.WorkingDir, err = ioutil.TempDir("", "sti")
				if err != nil {
					fmt.Println(err.Error())
					return
				}
				defer os.Remove(buildReq.WorkingDir)
			}

			sti.Build(buildReq)
		},
	}
	buildCmd.Flags().BoolVar(&(buildReq.Clean), "clean", false, "Perform a clean build")
	buildCmd.Flags().StringVar(&(req.WorkingDir), "dir", "tempdir", "Directory where generated Dockerfiles and other support scripts are created")
	buildCmd.Flags().StringVar(&(req.RuntimeImage), "runtime-image", "", "Set the runtime image to use")
	buildCmd.Flags().StringVarP(&envString, "env", "e", "", "Specify an environment var NAME=VALUE,NAME2=VALUE2,...")
	stiCmd.AddCommand(buildCmd)

	validateCmd := &cobra.Command{
		Use:   "validate BASE_IMAGE",
		Short: "Validate an image",
		Long:  "Validate an image and optional runtime image",
		Run: func(cmd *cobra.Command, args []string) {
			buildReq.BaseImage = args[0]
			res, err := sti.Validate(validateReq)

			if err != nil {
				fmt.Printf("An error occured: %s", err.Error())
				return
			}

			for _, message := range res.Messages {
				fmt.Println(message)
			}
		},
	}
	validateCmd.Flags().StringVarP(&(req.RuntimeImage), "runtime-image", "R", "", "Set the runtime image to use")
	validateCmd.Flags().BoolVarP(&(validateReq.Incremental), "incremental", "I", false, "Validate for an incremental build")
	stiCmd.AddCommand(validateCmd)

	stiCmd.Execute()
}
