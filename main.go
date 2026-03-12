package main

import (
	"os"

	"deployctl/cmd"
)

var version = "dev"

func main() {
	os.Exit(cmd.Execute(version))
}
