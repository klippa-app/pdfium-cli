package cmd

import (
	"fmt"
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
	Long:  "Explode a PDF into multiple PDFs, the output filename should contain a \"%d\" placeholder for the page number, e.g. split invoice.pdf invoice-%d.pdf, the result for 2-page PDF will be invoice-1.pdf and invoice-2.pdf.",
	Args: func(cmd *cobra.Command, args []string) error {
		if err := cobra.ExactArgs(2)(cmd, args); err != nil {
			return err
		}

		if _, err := os.Stat(args[0]); err != nil {
			return fmt.Errorf("could not open input file %s: %w", args[0], err)
		}

		if !strings.Contains(args[1], "%d") {
			return fmt.Errorf("output string %s should contain page pattern %%d", args[1])
		}

		return nil
	},
	Run: func(cmd *cobra.Command, args []string) {
		err := pdf.LoadPdfium()
		if err != nil {
			cmd.PrintErrf("could not load pdfium: %w", args[0], err)
			return
		}
		defer pdf.ClosePdfium()

		document, err := openFile(args[0])
		if err != nil {
			cmd.PrintErrf("could not open input file %s: %w", args[0], err)
			return
		}
		defer pdf.PdfiumInstance.FPDF_CloseDocument(&requests.FPDF_CloseDocument{Document: document.Document})

		pageCount, err := pdf.PdfiumInstance.FPDF_GetPageCount(&requests.FPDF_GetPageCount{
			Document: document.Document,
		})
		if err != nil {
			cmd.PrintErrf("could not get page count for PDF %s: %w", args[0], err)
			return
		}

		pageRange := "first-last"
		if cmd.Flag("pages").Value.String() != "" {
			pageRange = cmd.Flag("pages").Value.String()
		}

		parsedPageRange, _, err := pdf.NormalizePageRange(pageCount.PageCount, pageRange, false)
		if err != nil {
			cmd.PrintErrf("invalid page range '%s': %s", pageRange, err)
			return
		}

		splitPages := strings.Split(*parsedPageRange, ",")
		for _, page := range splitPages {
			newDocument, err := pdf.PdfiumInstance.FPDF_CreateNewDocument(&requests.FPDF_CreateNewDocument{})
			if err != nil {
				cmd.PrintErrf("could not create new document for page %s: %w", page, err)
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
				cmd.PrintErrf("could not import page %s into new document: %w", page, err)
				return
			}

			newFilePath := strings.Replace(args[1], "%d", page, -1)
			_, err = pdf.PdfiumInstance.FPDF_SaveAsCopy(&requests.FPDF_SaveAsCopy{
				Document: newDocument.Document,
				FilePath: &newFilePath,
			})
			if err != nil {
				closeFunc()
				cmd.PrintErrf("could not save document for page %s: %w", page, err)
				return
			}

			closeFunc()

			cmd.Printf("Exploded page %s into %s", page, newFilePath)
		}
	},
}
