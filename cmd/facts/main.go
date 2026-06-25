package main

import (
	"fmt"
	"io"
	"os"

	"github.com/ncode/facts/internal/app"
	"github.com/ncode/facts/internal/cli"
)

func main() {
	if code := runMain(os.Stdout, os.Stderr, os.Args[1:]); code != 0 {
		os.Exit(code)
	}
}

func runMain(stdout, stderr io.Writer, args []string) int {
	if err := app.Run(stdout, stderr, args); err != nil {
		if status, ok := err.(app.ExitStatus); ok {
			return status.Code()
		}
		if cli.IsOptionError(err) {
			fmt.Fprintf(stderr, "ERROR Facts::OptionsValidator - %v\n", err)
		} else {
			fmt.Fprintln(stderr, err)
		}
		return 1
	}
	return 0
}
