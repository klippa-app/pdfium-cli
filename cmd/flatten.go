package cmd

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/klippa-app/go-pdfium/responses"

	"github.com/klippa-app/pdfium-cli/pdf"

	"github.com/klippa-app/go-pdfium/requests"
	"github.com/spf13/cobra"
)

func init() {
	addGenericPDFOptions(flattenCmd)
	addPagesOption("The pages or page to flatten", flattenCmd)
	rootCmd.AddCommand(flattenCmd)
}

var flattenCmd = &cobra.Command{
	Use:   "flatten [input] [output]",
	Short: "Flatten a PDF",
	Long:  "Flatten a PDF.\n[input] can either be a file path or - for stdin.\n[output] can either be a file path or - for stdout. In the case of stdout, multiple files will be delimited by the value of the std-file-delimiter, with a newline before and after it. The output filename should contain a \"%d\" placeholder for the page number, e.g. split invoice.pdf invoice-%d.pdf, the result for a 2-page PDF will be invoice-1.pdf and invoice-2.pdf.",
	Args: func(cmd *cobra.Command, args []string) error {
		if err := cobra.ExactArgs(2)(cmd, args); err != nil {
			return newExitCodeError(err, ExitCodeInvalidArguments)
		}

		if err := validFile(args[0]); err != nil {
			return fmt.Errorf("could not open input file %s: %w\n", args[0], newExitCodeError(err, ExitCodeInvalidOutput))
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

		document, closeFile, err := openFile(args[0])
		if err != nil {
			handleError(cmd, fmt.Errorf("could not open input file %s: %w\n", args[0], err), ExitCodeInvalidInput)
			return
		}
		defer closeFile()

		pageCount, err := pdf.PdfiumInstance.FPDF_GetPageCount(&requests.FPDF_GetPageCount{
			Document: document.Document,
		})
		if err != nil {
			handleError(cmd, fmt.Errorf("could not get page count for PDF %s: %w\n", args[0], newPdfiumError(err)), ExitCodePdfiumError)
			return
		}

		pageRange := "first-last"
		if pages != "" {
			pageRange = pages
		}

		parsedPageRange, _, err := pdf.NormalizePageRange(pageCount.PageCount, pageRange, ignoreInvalidPages)
		if err != nil {
			handleError(cmd, fmt.Errorf("invalid page range '%s': %w\n", pageRange, err), ExitCodeInvalidPageRange)
			return
		}

		pages := strings.Split(*parsedPageRange, ",")
		for _, page := range pages {
			pageInt, _ := strconv.Atoi(page)
			page, err := pdf.PdfiumInstance.FPDF_LoadPage(&requests.FPDF_LoadPage{
				Document: document.Document,
				Index:    pageInt - 1, // pdfium is 0-index based
			})

			if err != nil {
				handleError(cmd, fmt.Errorf("could not load page for page %d for PDF %s: %w\n", pageInt, args[0], newPdfiumError(err)), ExitCodePdfiumError)
				return
			}

			closePageFunc := func() {
				pdf.PdfiumInstance.FPDF_ClosePage(&requests.FPDF_ClosePage{
					Page: page.Page,
				})
			}

			result, err := pdf.PdfiumInstance.FPDFPage_Flatten(&requests.FPDFPage_Flatten{
				Page: requests.Page{
					ByReference: &page.Page,
				},
				Usage: requests.FPDFPage_FlattenUsageNormalDisplay,
			})
			if err != nil {
				closePageFunc()
				handleError(cmd, fmt.Errorf("could not flatten page %d for PDF %s: %w\n", pageInt, args[0], newPdfiumError(err)), ExitCodePdfiumError)
				return
			}

			if result.Result == responses.FPDFPage_FlattenResultFail {
				closePageFunc()
				handleError(cmd, fmt.Errorf("could not flatten page %d for PDF %s: result was that the flattening failed\n", pageInt, args[0]), ExitCodePdfiumError)
				return
			}

			closePageFunc()
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
			Document:   document.Document,
			FileWriter: fileWriter,
		})
		if err != nil {
			handleError(cmd, fmt.Errorf("could not save new document: %w", newPdfiumError(err)), ExitCodePdfiumError)
			return
		}
	},
}
