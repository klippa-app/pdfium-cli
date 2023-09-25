package cmd

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/klippa-app/pdfium-cli/pdf"

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

var stdinDocuments [][]byte
var stdinNoMoreFiles = errors.New("no more files on stdin")

func openFile(filename string) (*responses.OpenDocument, func(), error) {
	openDocumentRequest := &requests.OpenDocument{}

	closeFile := func() {}

	// Support opening file from stdin.
	if filename == stdFilename {
		if stdinDocuments == nil {
			// @todo: possible improve this by reading up to the delimiter
			//   for every document and not all at once.
			readStdin, err := io.ReadAll(os.Stdin)
			if err != nil {
				return nil, nil, err
			}

			stdinDocuments = bytes.Split(readStdin, []byte("\n"+stdFileDelimiter+"\n"))
		}

		if len(stdinDocuments) == 0 {
			return nil, nil, stdinNoMoreFiles
		}

		stdinDocument := stdinDocuments[len(stdinDocuments)-1]
		stdinDocuments = stdinDocuments[:len(stdinDocuments)-1]

		// For stdin we need to read the full thing because pdfium doesn't
		// support streaming when it doesn't know the size of the file.
		reader := bytes.NewReader(stdinDocument)
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
		return nil, closeFile, fmt.Errorf("could not open file with pdfium: %w", newPdfiumError(err))
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

type pdfiumError struct {
	originalError error
}

func (e *pdfiumError) Error() string {
	return e.originalError.Error()
}

func newPdfiumError(err error) *pdfiumError {
	return &pdfiumError{
		originalError: err,
	}
}

type ExitCodeError struct {
	originalError error
	exitCode      int
}

func (e *ExitCodeError) Error() string {
	return e.originalError.Error()
}

func (e *ExitCodeError) ExitCode() int {
	return e.exitCode
}

func newExitCodeError(err error, code int) *ExitCodeError {
	return &ExitCodeError{
		originalError: err,
		exitCode:      code,
	}
}

func handleError(cmd *cobra.Command, err error, defaultCode int) {
	cmd.PrintErr(err)

	errorCode := defaultCode

	exitCodeError := &ExitCodeError{}
	if errors.As(err, &exitCodeError) {
		errorCode = exitCodeError.ExitCode()
	}

	target := &pdfiumError{}
	if errors.As(err, &target) {
		errorMsg := target.Error()
		if strings.HasPrefix(errorMsg, "1: ") {
			errorCode = ExitCodePdfiumUnknownError
		} else if strings.HasPrefix(errorMsg, "2: ") {
			errorCode = ExitCodePdfiumFileError
		} else if strings.HasPrefix(errorMsg, "3: ") {
			errorCode = ExitCodePdfiumBadFileError
		} else if strings.HasPrefix(errorMsg, "4: ") {
			errorCode = ExitCodePdfiumPasswordError
		} else if strings.HasPrefix(errorMsg, "5: ") {
			errorCode = ExitCodePdfiumSecurityError
		} else if strings.HasPrefix(errorMsg, "6: ") {
			errorCode = ExitCodePdfiumPageError
		}
	}

	os.Exit(errorCode)
}
