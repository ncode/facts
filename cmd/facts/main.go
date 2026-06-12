package main

import (
	"fmt"
	"os"

	"github.com/ncode/facts/internal/app"
	"github.com/ncode/facts/internal/cli"
)

func main() {
	if err := app.Run(os.Stdout, os.Stderr, os.Args[1:]); err != nil {
		if status, ok := err.(app.ExitStatus); ok {
			os.Exit(status.Code())
		}
		if cli.IsOptionError(err) {
			fmt.Fprintf(os.Stderr, "ERROR Facts::OptionsValidator - %v\n", err)
		} else {
			fmt.Fprintln(os.Stderr, err)
		}
		os.Exit(1)
	}
}
