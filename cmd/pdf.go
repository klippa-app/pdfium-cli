package cmd

import (
	"github.com/spf13/cobra"
)

var (
	// Used for flags.
	password string
	pages    string
	dpi      int
)

func addGenericPDFOptions(command *cobra.Command) {
	command.Flags().StringVarP(&password, "password", "", "", "Password on the input PDF file")
}

func addPagesOption(intro string, command *cobra.Command) {
	command.Flags().StringVarP(&pages, "pages", "p", "", intro+". Ranges are like '1-3,5', which will result in a PDF file with pages 1, 2, 3 and 5. You can use the keywords first and last. You can prepend a page number with r to start counting from the end. Examples: use '2-last' for the second page until the last page, use '3-r1' for page 3 until the second-last page.")
}

func addDPIOption(command *cobra.Command) {
	command.Flags().IntVarP(&dpi, "dpi", "", 200, "The DPI to render the image in")
}
