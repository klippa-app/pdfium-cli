package cmd

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/klippa-app/pdfium-cli/pdf"

	"github.com/klippa-app/go-pdfium/requests"
	"github.com/spf13/cobra"
)

func init() {
	addGenericPDFOptions(explodeCmd)
	addPagesOption("The pages or page ranges to use in the explode", explodeCmd)

	rootCmd.AddCommand(explodeCmd)
}

var explodeCmd = &cobra.Command{
	Use:   "explode [input] [output]",
	Short: "Explode a PDF into multiple PDFs",
	Long:  "Explode a PDF into multiple PDFs.\n[input] can either be a file path or - for stdin.\n[output] can either be a file path or - for stdout, stdout is only supported when the output will be one file. The output filename should contain a \"%d\" placeholder for the page number, e.g. split invoice.pdf invoice-%d.pdf, the result for a 2-page PDF will be invoice-1.pdf and invoice-2.pdf.",
	Args: func(cmd *cobra.Command, args []string) error {
		if err := cobra.ExactArgs(2)(cmd, args); err != nil {
			return err
		}

		if err := validFile(args[0]); err != nil {
			return fmt.Errorf("could not open input file %s: %w\n", args[0], err)
		}

		if args[1] != stdFilename && !strings.Contains(args[1], "%d") {
			return fmt.Errorf("output string %s should contain page pattern %%d\n", args[1])
		}

		return nil
	},
	Run: func(cmd *cobra.Command, args []string) {
		err := pdf.LoadPdfium()
		if err != nil {
			cmd.PrintErrf("could not load pdfium: %w\n", err)
			return
		}
		defer pdf.ClosePdfium()

		document, closeFile, err := openFile(args[0])
		if err != nil {
			cmd.PrintErrf("could not open input file %s: %w\n", args[0], err)
			return
		}
		defer closeFile()

		pageCount, err := pdf.PdfiumInstance.FPDF_GetPageCount(&requests.FPDF_GetPageCount{
			Document: document.Document,
		})
		if err != nil {
			cmd.PrintErrf("could not get page count for PDF %s: %w\n", args[0], err)
			return
		}

		pageRange := "first-last"
		if cmd.Flag("pages").Value.String() != "" {
			pageRange = cmd.Flag("pages").Value.String()
		}

		parsedPageRange, _, err := pdf.NormalizePageRange(pageCount.PageCount, pageRange, false)
		if err != nil {
			cmd.PrintErrf("invalid page range '%s': %s\n", pageRange, err)
			return
		}

		splitPages := strings.Split(*parsedPageRange, ",")

		if args[1] == stdFilename && len(splitPages) > 1 {
			cmd.PrintErrf("could not explode into multiple pages with output to stdout\n")
			return
		}

		for _, page := range splitPages {
			newDocument, err := pdf.PdfiumInstance.FPDF_CreateNewDocument(&requests.FPDF_CreateNewDocument{})
			if err != nil {
				cmd.PrintErrf("could not create new document for page %s: %w\n", page, err)
				return
			}

			closeFunc := func() {
				pdf.PdfiumInstance.FPDF_CloseDocument(&requests.FPDF_CloseDocument{Document: newDocument.Document})
			}

			_, err = pdf.PdfiumInstance.FPDF_ImportPages(&requests.FPDF_ImportPages{
				Source:      document.Document,
				Destination: newDocument.Document,
				PageRange:   &page,
			})
			if err != nil {
				closeFunc()
				cmd.PrintErrf("could not import page %s into new document: %w\n", page, err)
				return
			}

			newFilePath := strings.Replace(args[1], "%d", page, -1)

			var fileWriter io.Writer
			if args[1] == stdFilename {
				fileWriter = os.Stdout
			} else {
				createdFile, err := os.Create(newFilePath)
				if err != nil {
					cmd.PrintErrf("could not save document for page %s: %w\n", page, err)
					return
				}
				fileWriter = createdFile
				originalCloseFunc := closeFunc
				closeFunc = func() {
					originalCloseFunc()
					createdFile.Close()
				}
			}

			_, err = pdf.PdfiumInstance.FPDF_SaveAsCopy(&requests.FPDF_SaveAsCopy{
				Document:   newDocument.Document,
				FileWriter: fileWriter,
			})
			if err != nil {
				closeFunc()
				cmd.PrintErrf("could not save document for page %s: %w\n", page, err)
				return
			}

			closeFunc()

			if args[1] != stdFilename {
				cmd.Printf("Exploded page %s into %s\n", page, newFilePath)
			}
		}
	},
}
