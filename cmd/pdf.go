package cmd

import (
	"github.com/klippa-app/pdfium-cli/pdf"

	"github.com/klippa-app/go-pdfium/requests"
	"github.com/klippa-app/go-pdfium/responses"
	"github.com/spf13/cobra"
)

var (
	// Used for flags.
	password string
	pages    string
)

func addGenericPDFOptions(command *cobra.Command) {
	command.Flags().StringVarP(&password, "password", "p", "", "Password on the input PDF file")
}

func addPagesOption(intro string, command *cobra.Command) {
	command.Flags().StringVarP(&pages, "pages", "", "first-last", intro+". Ranges are like '1-3,5', which will result in a PDF file with pages 1, 2, 3 and 5. You can use the keywords first and last. You can prepend a page number with r to start counting from the end. Examples: use '2-last' for the second page until the last page, use '3-r1' for page 3 until the second-last page.")
}

func openFile(path string) (*responses.OpenDocument, error) {
	openDocumentRequest := &requests.OpenDocument{
		FilePath: &path,
	}

	if password != "" {
		openDocumentRequest.Password = &password
	}

	return pdf.PdfiumInstance.OpenDocument(openDocumentRequest)
}
