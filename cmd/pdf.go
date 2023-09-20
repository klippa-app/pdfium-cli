package cmd

import (
	"bytes"
	"github.com/klippa-app/pdfium-cli/pdf"
	"io"
	"os"
	"strings"

	"github.com/klippa-app/go-pdfium/requests"
	"github.com/klippa-app/go-pdfium/responses"
	"github.com/spf13/cobra"
)

var (
	// Used for flags.
	password         string
	stdFileDelimiter string
	pages            string
)

func addGenericPDFOptions(command *cobra.Command) {
	command.Flags().StringVarP(&password, "password", "p", "", "Password on the input PDF file(s).")
	command.Flags().StringVarP(&stdFileDelimiter, "std-file-delimiter", "", "--pdfium-cli-file-boundary", "The delimiter to use when having multiple files in your input and/or output.")
}

func addPagesOption(intro string, command *cobra.Command) {
	command.Flags().StringVarP(&pages, "pages", "", "first-last", intro+". Ranges are like '1-3,5', which will result in a PDF file with pages 1, 2, 3 and 5. You can use the keywords first and last. You can prepend a page number with r to start counting from the end. Examples: use '2-last' for the second page until the last page, use '3-r1' for page 3 until the second-last page.")
}

func isExperimentalError(err error) bool {
	return strings.Contains(err.Error(), "pdfium_experimental")
}

const stdFilename = "-"

func openFile(filename string) (*responses.OpenDocument, func(), error) {
	openDocumentRequest := &requests.OpenDocument{}

	closeFile := func() {}

	// Support opening file from stdin.
	if filename == stdFilename {
		// For stdin we need to read the full thing because pdfium doesn't
		// support streaming when it doesn't know the size of the file.
		readStdin, err := io.ReadAll(os.Stdin)
		if err != nil {
			return nil, nil, err
		}
		reader := bytes.NewReader(readStdin)
		openDocumentRequest.FileReader = reader
		openDocumentRequest.FileReaderSize = reader.Size()
	} else {
		file, err := os.Open(filename)
		if err != nil {
			return nil, nil, err
		}

		closeFile = func() {
			file.Close()
		}

		// For files we stat the file so that we can tell pdfium the file size
		// which it will need to do proper seeking in the file.
		fileStat, err := file.Stat()
		if err != nil {
			return nil, closeFile, nil
		}

		openDocumentRequest.FileReader = file
		openDocumentRequest.FileReaderSize = fileStat.Size()
	}

	if password != "" {
		openDocumentRequest.Password = &password
	}

	openedDocument, err := pdf.PdfiumInstance.OpenDocument(openDocumentRequest)
	if err != nil {
		return nil, closeFile, err
	}

	originalCloseFile := closeFile
	closeFile = func() {
		pdf.PdfiumInstance.FPDF_CloseDocument(&requests.FPDF_CloseDocument{Document: openedDocument.Document})
		originalCloseFile()
	}

	return openedDocument, closeFile, nil
}

func validFile(filename string) error {
	if filename == stdFilename {
		return nil
	}

	if _, err := os.Stat(filename); err != nil {
		return err
	}

	return nil
}
