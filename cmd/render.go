package cmd

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/klippa-app/go-pdfium/requests"
	"github.com/klippa-app/pdfium-cli/pdf"
	"github.com/spf13/cobra"
)

func init() {
	addGenericPDFOptions(renderCmd)
	addPagesOption("The pages or page ranges to render", renderCmd)
	addDPIOption(renderCmd)

	rootCmd.AddCommand(renderCmd)
}

var renderCmd = &cobra.Command{
	Use:   "render [input] [output]",
	Short: "Render a PDF into images",
	Long:  "Render a PDF into images, the output filename should contain a \"%d\" placeholder for the page number, e.g. split invoice.pdf invoice-%d.jpg, the result for 2-page PDF will be invoice-1.jpg and invoice-2.jpg.",
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

		document, err := pdf.PdfiumInstance.OpenDocument(&requests.OpenDocument{
			FilePath: &args[0],
		})

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
			pageInt, _ := strconv.Atoi(page)
			renderPageRequest := []requests.RenderPageInDPI{
				{
					Page: requests.Page{
						ByIndex: &requests.PageByIndex{
							Document: document.Document,
							Index:    pageInt - 1, // pdfium is 0-index based
						},
					},
					DPI: dpi,
				},
			}

			newFilePath := strings.Replace(args[1], "%d", page, -1)

			_, err := pdf.PdfiumInstance.RenderToFile(&requests.RenderToFile{
				RenderPagesInDPI: &requests.RenderPagesInDPI{
					Pages: renderPageRequest,
				},
				OutputFormat:   requests.RenderToFileOutputFormatJPG,
				OutputTarget:   requests.RenderToFileOutputTargetFile,
				TargetFilePath: newFilePath,
				MaxFileSize:    20 * 1024 * 1014,
			})
			if err != nil {
				cmd.PrintErrf("could not render page %s into image: %w", page, err)
				return
			}

			cmd.Printf("Rendered page %s into %s", page, newFilePath)
		}
	},
}
