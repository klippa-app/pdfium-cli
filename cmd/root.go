package cmd

import (
	"os"

	"github.com/klippa-app/pdfium-cli/version"

	"github.com/spf13/cobra"
)

var (
	// Used for flags.
	//cfgFile string

	rootCmd = &cobra.Command{
		Use:     "pdfium",
		Short:   "A CLI tool to use pdfium",
		Long:    `pdfium-cli is a CLI tool that allows you to use pdfium from the CLI`,
		Version: version.VERSION,
	}
)

// Execute executes the root command.
func Execute() error {
	rootCmd.SetOut(os.Stdout)
	return rootCmd.Execute()
}

func init() {
	//rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.cobra.yaml)")

	//rootCmd.AddCommand(addCmd)
	//rootCmd.AddCommand(initCmd)
}
