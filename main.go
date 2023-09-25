package main

import (
	"errors"
	"github.com/klippa-app/pdfium-cli/cmd"
	"os"
)

func main() {
	err := cmd.Execute()
	if err != nil {
		exitCodeError := &cmd.ExitCodeError{}
		if errors.As(err, &exitCodeError) {
			os.Exit(exitCodeError.ExitCode())
		} else {
			os.Exit(cmd.ExitCodeInvalidArguments)
		}
	}
}
