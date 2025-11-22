package main

import (
	"os"

	"github.com/lallihdp-web/go-query-analyzer/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
