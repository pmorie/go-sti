package main

import (
	"github.com/pmorie/go-sti/cmd"
	_ "net/http/pprof"
)

func main() {
	cmd.Execute()
}
