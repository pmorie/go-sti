package cmd

import (
	"github.com/pmorie/go-sti/sti"
	"github.com/smarterclayton/cobra"
)

func Execute() {
	var (
		help bool
		req  sti.Request
	)

	buildReq := sti.BuildRequest{Request: req}
	validateReq := sti.ValidateRequest{Request: req}

	stiCmd := &cobra.Command{
		Use:   "sti",
		Short: "STI is a tool for building repeatable docker images",
		Long: `A command-line interface for the sti library
              Complete documentation is available at http://github.com/pmorie/go-sti`,
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Usage()
		},
	}
	stiCmd.Flags().BoolVarP(&help, "help", "h", false, "Get help")
	stiCmd.PersistentFlags().StringVarP(&(req.DockerSocket), "url", "U", "unix:///var/run/docker.sock", "Set the url of the docker socket to use")
	stiCmd.PersistentFlags().IntVarP(&(req.DockerTimeout), "timeout", "T", 30, "Set the timeout for docker operations")

	buildCmd := &cobra.Command{
		Use:   "build SOURCE BASE_IMAGE TAG",
		Short: "Build an image",
		Long:  "Build an image",
		Run: func(cmd *cobra.Command, args []string) {

			sti.Build(buildReq)
		},
	}
	buildCmd.Flags().BoolVarP(&(buildReq.Clean), "clean", "C", false, "Perform a clean build")
	buildCmd.Flags().StringVarP(&(req.RuntimeImage), "runtime-image", "R", "", "Set the runtime image to use")
	stiCmd.AddCommand(buildCmd)

	validateCmd := &cobra.Command{
		Use:   "validate BASE_IMAGE",
		Short: "Validate an image",
		Long:  "Validate an image and optional runtime image",
		Run: func(cmd *cobra.Command, args []string) {
			sti.Validate(validateReq)
		},
	}
	validateCmd.Flags().StringVarP(&(req.RuntimeImage), "runtime-image", "R", "", "Set the runtime image to use")
	validateCmd.Flags().BoolVarP(&(validateReq.Incremental), "incremental", "I", false, "Validate for an incremental build")
	stiCmd.AddCommand(validateCmd)

	stiCmd.Execute()
}
