package cmd

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/klippa-app/pdfium-cli/pdf"

	"github.com/klippa-app/go-pdfium/enums"
	"github.com/klippa-app/go-pdfium/requests"
	"github.com/spf13/cobra"
)

var (
	// Used for flags.
	dpi               int
	fileType          string
	maxFileSize       int64
	combinePages      bool
	maxWidth          int
	maxHeight         int
	padding           int
	quality           int
	progressive       bool
	renderAnnotations bool
	renderForm        bool
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
	renderCmd.Flags().IntVarP(&quality, "quality", "", 95, "The quality to render the image in, only used for jpeg. The option max-file-size may lower this if necessary.")
	renderCmd.Flags().BoolVarP(&progressive, "progressive", "", false, "Create progressive images, only used for jpeg.")
	renderCmd.Flags().BoolVarP(&renderAnnotations, "render-annotations", "", false, "Render annotations that are embedded in the PDF.")
	renderCmd.Flags().BoolVarP(&renderForm, "render-form", "", false, "Render form fields that are embedded in the PDF.")

	rootCmd.AddCommand(renderCmd)
}

var renderCmd = &cobra.Command{
	Use:   "render [input] [output]",
	Short: "Render a PDF into images",
	Long:  "Render a PDF into images.\n[input] can either be a file path or - for stdin.\n[output] can either be a file path or - for stdout.  or - for stdout. In the case of stdout, multiple files will be delimited by the value of the std-file-delimiter, with a newline before and after it. The output filename should contain a \"%d\" placeholder for the page number when rendering more than one page and when not using the combine-pages option, e.g. render invoice.pdf invoice-%d.jpg, the result for a 2-page PDF will be invoice-1.jpg and invoice-2.jpg.",
	Args: func(cmd *cobra.Command, args []string) error {
		if err := cobra.ExactArgs(2)(cmd, args); err != nil {
			return newExitCodeError(err, ExitCodeInvalidArguments)
		}

		if err := validFile(args[0]); err != nil {
			return fmt.Errorf("could not open input file %s: %w", args[0], newExitCodeError(err, ExitCodeInvalidInput))
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

		renderPages := []requests.Page{}
		splitPages := strings.Split(*parsedPageRange, ",")

		if len(splitPages) > 1 && !combinePages {
			if args[1] != stdFilename && !strings.Contains(args[1], "%d") {
				handleError(cmd, fmt.Errorf("output string %s should contain page pattern %%d\n", args[1]), ExitCodeInvalidArguments)
				return
			}
		}

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
			handleError(cmd, fmt.Errorf("invalid file type: %s\n", fileType), ExitCodeInvalidArguments)
			return
		}

		renderFlags := enums.FPDF_RENDER_FLAG(0)
		if renderAnnotations {
			renderFlags = enums.FPDF_RENDER_FLAG_ANNOT
		}

		if combinePages {
			renderRequest := &requests.RenderToFile{
				OutputFormat:  outputFormat,
				MaxFileSize:   maxFileSize,
				OutputQuality: quality,
				Progressive:   progressive,
			}

			if args[1] == stdFilename {
				renderRequest.OutputTarget = requests.RenderToFileOutputTargetBytes
			} else {
				renderRequest.OutputTarget = requests.RenderToFileOutputTargetFile
				renderRequest.TargetFilePath = args[1]
			}

			if maxWidth > 0 || maxHeight > 0 {
				renderPagesInPixels := []requests.RenderPageInPixels{}
				for _, renderPage := range renderPages {
					renderPagesInPixels = append(renderPagesInPixels, requests.RenderPageInPixels{
						Page:        renderPage,
						Width:       maxWidth,
						Height:      maxHeight,
						RenderFlags: renderFlags,
						RenderForm:  renderForm,
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
						Page:        renderPage,
						DPI:         dpi,
						RenderFlags: renderFlags,
						RenderForm:  renderForm,
					})
				}
				renderRequest.RenderPagesInDPI = &requests.RenderPagesInDPI{
					Pages:   renderPagesInDPI,
					Padding: padding,
				}
			}

			result, err := pdf.PdfiumInstance.RenderToFile(renderRequest)
			if err != nil {
				handleError(cmd, fmt.Errorf("could not render pages %s into image: %w\n", *parsedPageRange, newPdfiumError(err)), ExitCodePdfiumError)
				return
			}

			if args[1] != stdFilename {
				cmd.Println(fmt.Errorf("Rendered pages %s into %s\n", *parsedPageRange, args[1]))
			} else {
				_, err = os.Stdout.Write(*result.ImageBytes)
				if err != nil {
					handleError(cmd, fmt.Errorf("could not render pages %s into image: %w\n", *parsedPageRange, err), ExitCodeInvalidOutput)
					return
				}
			}
		} else {
			for i, renderPage := range renderPages {
				page := strconv.Itoa(renderPage.ByIndex.Index + 1)
				newFilePath := strings.Replace(args[1], "%d", page, -1)

				renderRequest := &requests.RenderToFile{
					OutputFormat:  outputFormat,
					MaxFileSize:   maxFileSize,
					OutputQuality: quality,
					Progressive:   progressive,
				}

				if args[1] == stdFilename {
					renderRequest.OutputTarget = requests.RenderToFileOutputTargetBytes
				} else {
					renderRequest.OutputTarget = requests.RenderToFileOutputTargetFile
					renderRequest.TargetFilePath = newFilePath
				}

				if maxWidth > 0 || maxHeight > 0 {
					renderRequest.RenderPagesInPixels = &requests.RenderPagesInPixels{
						Pages: []requests.RenderPageInPixels{
							{
								Page:        renderPage,
								Width:       maxWidth,
								Height:      maxHeight,
								RenderFlags: renderFlags,
								RenderForm:  renderForm,
							},
						},
						Padding: padding,
					}
				} else {
					renderRequest.RenderPagesInDPI = &requests.RenderPagesInDPI{
						Pages: []requests.RenderPageInDPI{
							{
								Page:        renderPage,
								DPI:         dpi,
								RenderFlags: renderFlags,
								RenderForm:  renderForm,
							},
						},
						Padding: padding,
					}
				}

				result, err := pdf.PdfiumInstance.RenderToFile(renderRequest)
				if err != nil {
					handleError(cmd, fmt.Errorf("could not render page %s into image: %w\n", page, newPdfiumError(err)), ExitCodePdfiumError)
					return
				}

				if args[1] != stdFilename {
					cmd.Printf("Rendered page %s into %s\n", page, newFilePath)
				} else {
					if i > 0 {
						os.Stdout.WriteString("\n")
						os.Stdout.WriteString(stdFileDelimiter)
						os.Stdout.WriteString("\n")
					}
					_, err = os.Stdout.Write(*result.ImageBytes)
					if err != nil {
						handleError(cmd, fmt.Errorf("could not render page %s into image: %w\n", page, err), ExitCodeInvalidOutput)
						return
					}
				}
			}
		}
	},
}
