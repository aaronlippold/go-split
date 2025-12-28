// go-split: CLI tool to intelligently split large Go files into smaller modules.
package main

import (
	"fmt"
	"os"

	"github.com/aaronlippold/go-split/internal/cli"
)

func main() {
	if err := cli.Run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
