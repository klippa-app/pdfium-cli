package cmd

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/klippa-app/go-pdfium/requests"

	"github.com/klippa-app/pdfium-cli/pdf"
	"github.com/spf13/cobra"
)

// parseFileWithPageRange parses a file argument that may contain an optional page range
// in the format "filename.pdf[1-3]". Returns the filename and page range string.
// If no page range is specified, returns "first-last" as the page range.
func parseFileWithPageRange(arg string) (string, string) {
	if strings.HasSuffix(arg, "]") {
		bracketIdx := strings.LastIndex(arg, "[")
		if bracketIdx != -1 {
			pageRange := arg[bracketIdx+1 : len(arg)-1]
			if pageRange != "" {
				return arg[:bracketIdx], pageRange
			}
		}
	}
	return arg, "first-last"
}

func init() {
	addGenericPDFOptions(mergeCmd)
	addIgnoreInvalidPagesOption(mergeCmd)
	rootCmd.AddCommand(mergeCmd)
}

var mergeCmd = &cobra.Command{
	Use:   "merge [input] ([input]...) [output]",
	Short: "Merge multiple PDFs into a single PDF",
	Long:  "Merge multiple PDFs into a single PDF.\n[output] can either be a file path or - for stdout.\nEach [input] can optionally include a page range using the syntax filename.pdf[{pagerange}],\nfor example invoice.pdf[1-3] to include only pages 1, 2 and 3.",
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) < 1 {
			return newExitCodeError(errors.New("no input given"), ExitCodeInvalidArguments)
		}
		if args[0] == stdFilename {
			if err := cobra.MinimumNArgs(1)(cmd, args); err != nil {
				return newExitCodeError(err, ExitCodeInvalidArguments)
			}
		} else {
			if err := cobra.MinimumNArgs(2)(cmd, args); err != nil {
				return newExitCodeError(err, ExitCodeInvalidArguments)
			}

			for i := 0; i < len(args)-1; i++ {
				filename, _ := parseFileWithPageRange(args[i])
				if _, err := os.Stat(filename); err != nil {
					return fmt.Errorf("could not open input file %s: %w", filename, newExitCodeError(err, ExitCodeInvalidInput))
				}
			}
		}

		return nil
	},
	Run: func(cmd *cobra.Command, args []string) {
		err := pdf.LoadPdfium()
		if err != nil {
			handleError(cmd, fmt.Errorf("could not load pdfium: %w\n", newPdfiumError(err)), ExitCodePdfiumError)
			return
		}
		defer pdf.ClosePdfium()

		newDocument, err := pdf.PdfiumInstance.FPDF_CreateNewDocument(&requests.FPDF_CreateNewDocument{})
		if err != nil {
			handleError(cmd, fmt.Errorf("could not create new document: %w", newPdfiumError(err)), ExitCodePdfiumError)
			return
		}

		mergedPageCount := 0
		i := 0
		for true {
			var filename string
			var filePageRange string
			if args[0] == stdFilename {
				filename = stdFilename
				filePageRange = "first-last"
			} else {
				// Reached last file.
				if i == len(args)-1 {
					break
				}
				filename, filePageRange = parseFileWithPageRange(args[i])
			}

			document, closeFile, err := openFile(filename)
			if err != nil {
				// Reached last stdin file.
				if err == stdinNoMoreFiles {
					break
				}
				handleError(cmd, fmt.Errorf("could not open input file %s: %w\n", filename, err), ExitCodeInvalidInput)
				return
			}

			closeFunc := func() {
				closeFile()
			}

			pageCount, err := pdf.PdfiumInstance.FPDF_GetPageCount(&requests.FPDF_GetPageCount{
				Document: document.Document,
			})
			if err != nil {
				closeFunc()
				handleError(cmd, fmt.Errorf("could not get page ranges for file %s: %w", filename, newPdfiumError(err)), ExitCodePdfiumError)
				return
			}

			pageRange, calculatedPageCount, err := pdf.NormalizePageRange(pageCount.PageCount, filePageRange, ignoreInvalidPages)
			if err != nil {
				closeFunc()
				handleError(cmd, fmt.Errorf("invalid page range '%s' for file %s: %w\n", filePageRange, filename, err), ExitCodeInvalidPageRange)
				return
			}

			_, err = pdf.PdfiumInstance.FPDF_ImportPages(&requests.FPDF_ImportPages{
				Source:      document.Document,
				Destination: newDocument.Document,
				PageRange:   pageRange,
				Index:       mergedPageCount,
			})
			if err != nil {
				closeFunc()
				handleError(cmd, fmt.Errorf("could not import pages for file %s: %w", filename, newPdfiumError(err)), ExitCodePdfiumError)
				return
			}

			mergedPageCount += *calculatedPageCount

			_, err = pdf.PdfiumInstance.FPDF_CloseDocument(&requests.FPDF_CloseDocument{
				Document: document.Document,
			})
			if err != nil {
				closeFunc()
				handleError(cmd, fmt.Errorf("could not close document for file %s: %w", filename, newPdfiumError(err)), ExitCodePdfiumError)
				return
			}

			closeFunc()
			i++
		}

		var fileWriter io.Writer
		if args[len(args)-1] == stdFilename {
			fileWriter = os.Stdout
		} else {
			createdFile, err := os.Create(args[len(args)-1])
			if err != nil {
				handleError(cmd, fmt.Errorf("could not save document: %w", err), ExitCodeInvalidOutput)
				return
			}

			defer createdFile.Close()
			fileWriter = createdFile
		}

		_, err = pdf.PdfiumInstance.FPDF_SaveAsCopy(&requests.FPDF_SaveAsCopy{
			Document:   newDocument.Document,
			FileWriter: fileWriter,
		})
		if err != nil {
			handleError(cmd, fmt.Errorf("could not save new document: %w", newPdfiumError(err)), ExitCodePdfiumError)
			return
		}

		_, err = pdf.PdfiumInstance.FPDF_CloseDocument(&requests.FPDF_CloseDocument{
			Document: newDocument.Document,
		})
		if err != nil {
			handleError(cmd, fmt.Errorf("could not save new document: %w", newPdfiumError(err)), ExitCodePdfiumError)
			return
		}
	},
}
