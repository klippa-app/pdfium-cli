package cmd

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/klippa-app/pdfium-cli/pdf"

	"github.com/klippa-app/go-pdfium/requests"
	"github.com/spf13/cobra"
)

var (
	// Used for flags.
	dpi          int
	fileType     string
	maxFileSize  int64
	combinePages bool
	maxWidth     int
	maxHeight    int
	padding      int
)

func init() {
	addGenericPDFOptions(renderCmd)
	addPagesOption("The pages or page ranges to render", renderCmd)

	renderCmd.Flags().IntVarP(&dpi, "dpi", "", 200, "The DPI to render the image in")
	renderCmd.Flags().StringVarP(&fileType, "file-type", "", "jpeg", "The file type to render in, jpeg or png")
	renderCmd.Flags().Int64VarP(&maxFileSize, "max-file-size", "", 0, "The maximum file size in bytes for the image, if the rendered image will be larger than this, we will try to compress it until it fits")
	renderCmd.Flags().BoolVarP(&combinePages, "combine-pages", "", false, "Combine pages in one image")
	renderCmd.Flags().IntVarP(&maxWidth, "max-width", "", 0, "The maximum width of the resulting image, this will disable the DPI option. The aspect ratio will be kept. When only the width is given, the height will be calculated automatically.")
	renderCmd.Flags().IntVarP(&maxHeight, "max-height", "", 0, "The maximum height of the resulting image, this will disable the DPI option. The aspect ratio will be kept. When only the height is given, the width will be calculated automatically.")
	renderCmd.Flags().IntVarP(&padding, "padding", "", 0, "The padding in pixels between pages when combining pages.")

	rootCmd.AddCommand(renderCmd)
}

var renderCmd = &cobra.Command{
	Use:   "render [input] [output]",
	Short: "Render a PDF into images",
	Long:  "Render a PDF into images, the output filename should contain a \"%d\" placeholder for the page number when rendering more than one page and when not using the combine-pages option, e.g. render invoice.pdf invoice-%d.jpg, the result for a 2-page PDF will be invoice-1.jpg and invoice-2.jpg.",
	Args: func(cmd *cobra.Command, args []string) error {
		if err := cobra.ExactArgs(2)(cmd, args); err != nil {
			return err
		}

		if _, err := os.Stat(args[0]); err != nil {
			return fmt.Errorf("could not open input file %s: %w", args[0], err)
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

		if pageCount.PageCount > 1 && !combinePages {
			if !strings.Contains(args[1], "%d") {
				cmd.PrintErrf("output string %s should contain page pattern %%d", args[1])
				return
			}
		}

		pageRange := "first-last"
		if pages != "" {
			pageRange = pages
		}

		parsedPageRange, _, err := pdf.NormalizePageRange(pageCount.PageCount, pageRange, false)
		if err != nil {
			cmd.PrintErrf("invalid page range '%s': %s", pageRange, err)
			return
		}

		renderPages := []requests.Page{}
		splitPages := strings.Split(*parsedPageRange, ",")
		for _, page := range splitPages {
			pageInt, _ := strconv.Atoi(page)
			renderPages = append(renderPages, requests.Page{
				ByIndex: &requests.PageByIndex{
					Document: document.Document,
					Index:    pageInt - 1, // pdfium is 0-index based
				},
			})
		}

		outputFormat := requests.RenderToFileOutputFormatJPG
		if fileType == "jpeg" {
			outputFormat = requests.RenderToFileOutputFormatJPG
		} else if fileType == "png" {
			outputFormat = requests.RenderToFileOutputFormatPNG
		} else {
			cmd.PrintErrf("invalid file type: %s", fileType)
			return
		}

		if combinePages {
			renderRequest := &requests.RenderToFile{
				OutputFormat:   outputFormat,
				OutputTarget:   requests.RenderToFileOutputTargetFile,
				TargetFilePath: args[1],
				MaxFileSize:    maxFileSize,
			}

			if maxWidth > 0 || maxHeight > 0 {
				renderPagesInPixels := []requests.RenderPageInPixels{}
				for _, renderPage := range renderPages {
					renderPagesInPixels = append(renderPagesInPixels, requests.RenderPageInPixels{
						Page:   renderPage,
						Width:  maxWidth,
						Height: maxHeight,
					})
				}
				renderRequest.RenderPagesInPixels = &requests.RenderPagesInPixels{
					Pages:   renderPagesInPixels,
					Padding: padding,
				}
			} else {
				renderPagesInDPI := []requests.RenderPageInDPI{}
				for _, renderPage := range renderPages {
					renderPagesInDPI = append(renderPagesInDPI, requests.RenderPageInDPI{
						Page: renderPage,
						DPI:  dpi,
					})
				}
				renderRequest.RenderPagesInDPI = &requests.RenderPagesInDPI{
					Pages:   renderPagesInDPI,
					Padding: padding,
				}
			}

			_, err := pdf.PdfiumInstance.RenderToFile(renderRequest)
			if err != nil {
				cmd.PrintErrf("could not render pages %s into image: %w", *parsedPageRange, err)
				return
			}

			cmd.Printf("Rendered pages %s into %s", *parsedPageRange, args[1])
		} else {
			for _, renderPage := range renderPages {
				page := strconv.Itoa(renderPage.ByIndex.Index + 1)

				newFilePath := strings.Replace(args[1], "%d", page, -1)

				renderRequest := &requests.RenderToFile{
					OutputFormat:   outputFormat,
					OutputTarget:   requests.RenderToFileOutputTargetFile,
					TargetFilePath: newFilePath,
					MaxFileSize:    maxFileSize,
				}

				if maxWidth > 0 || maxHeight > 0 {
					renderRequest.RenderPagesInPixels = &requests.RenderPagesInPixels{
						Pages: []requests.RenderPageInPixels{
							{
								Page:   renderPage,
								Width:  maxWidth,
								Height: maxHeight,
							},
						},
						Padding: padding,
					}
				} else {
					renderRequest.RenderPagesInDPI = &requests.RenderPagesInDPI{
						Pages: []requests.RenderPageInDPI{
							{
								Page: renderPage,
								DPI:  dpi,
							},
						},
						Padding: padding,
					}
				}

				_, err := pdf.PdfiumInstance.RenderToFile(renderRequest)
				if err != nil {
					cmd.PrintErrf("could not render page %s into image: %w", page, err)
					return
				}

				cmd.Printf("Rendered page %s into %s", page, newFilePath)
			}
		}
	},
}
