// go-split: CLI tool to intelligently split large Go files into smaller modules.
package main

import (
	"os"

	"github.com/aaronlippold/go-split/internal/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
