package main

import (
	"os"

	"github.com/lallihdp-web/codereview/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
