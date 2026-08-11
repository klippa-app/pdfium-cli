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
	cropPixels        string
	cropPoints        string
	cropRelative      string
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
	renderCmd.Flags().StringVarP(&cropPixels, "crop-px", "", "", "Render only a region of the page instead of the whole page, given as \"x,y,width,height\" in pixels of the full page as it would be rendered in the given dpi, for example --crop-px \"1000,500,800,600\". The origin is the top-left corner of the page. Can only be used when rendering a single page, use the pages option to select the page.")
	renderCmd.Flags().StringVarP(&cropPoints, "crop-points", "", "", "The same as crop-px, but in points, where one point is 1/72 inch. This is the unit that the info command reports page sizes in, for example --crop-points \"36,36,144,72\".")
	renderCmd.Flags().StringVarP(&cropRelative, "crop-relative", "", "", "The same as crop-px, but as a fraction of the page size, where 1 is the full width or height of the page, for example --crop-relative \"0.25,0.1,0.5,0.2\".")

	rootCmd.AddCommand(renderCmd)
}

var renderCmd = &cobra.Command{
	Use:   "render [input] [output]",
	Short: "Render a PDF into images",
	Long:  "Render a PDF into images.\n[input] can either be a file path or - for stdin.\n[output] can either be a file path or - for stdout.  or - for stdout. In the case of stdout, multiple files will be delimited by the value of the std-file-delimiter, with a newline before and after it. The output filename should contain a \"%d\" placeholder for the page number when rendering more than one page and when not using the combine-pages option, e.g. render invoice.pdf invoice-%d.jpg, the result for a 2-page PDF will be invoice-1.jpg and invoice-2.jpg.\nUse one of the crop options to render only a region of a page instead of the whole page, e.g. render blueprint.pdf detail.jpg --pages 2 --crop-points \"36,36,144,72\" renders a region of 144 by 72 points, 36 points from the left and the top of page 2. The region is rendered directly in the requested resolution, it is not cut out of a render of the full page. The dpi, max-width and max-height options apply to the region instead of to the full page, so --crop-px \"1000,500,800,600\" --max-width 1600 gives an image of 1600 pixels wide of that region. A region may run past the edges of the page, the part that falls outside of the page gets the background color, which makes it possible to cut a page into equally sized tiles.",
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
		var parsedCrop *pdf.PageCrop
		cropUnit := pdf.CropUnitPixels

		givenCrops := map[string]pdf.CropUnit{}
		if cropPixels != "" {
			givenCrops["crop-px"] = pdf.CropUnitPixels
		}
		if cropPoints != "" {
			givenCrops["crop-points"] = pdf.CropUnitPoints
		}
		if cropRelative != "" {
			givenCrops["crop-relative"] = pdf.CropUnitRelative
		}

		if len(givenCrops) > 1 {
			handleError(cmd, fmt.Errorf("only one of the crop-px, crop-points and crop-relative options can be used at the same time\n"), ExitCodeInvalidArguments)
			return
		}

		for cropOption, unit := range givenCrops {
			cropValue := cropPixels
			if unit == pdf.CropUnitPoints {
				cropValue = cropPoints
			} else if unit == pdf.CropUnitRelative {
				cropValue = cropRelative
			}

			crop, err := pdf.ParsePageCrop(cropValue)
			if err != nil {
				handleError(cmd, fmt.Errorf("invalid %s '%s': %w\n", cropOption, cropValue, err), ExitCodeInvalidArguments)
				return
			}

			parsedCrop = crop
			cropUnit = unit
		}

		if parsedCrop != nil && combinePages {
			handleError(cmd, fmt.Errorf("the crop options can not be used together with the combine-pages option\n"), ExitCodeInvalidArguments)
			return
		}

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

		if parsedCrop != nil && len(splitPages) > 1 {
			handleError(cmd, fmt.Errorf("the crop options can only be used when rendering a single page, the page range '%s' resolves to %d pages, use the pages option to select one page, for example --pages 1\n", pageRange, len(splitPages)), ExitCodeInvalidArguments)
			return
		}

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

		// The crop options are all converted to points here, which is the unit
		// that pdfium renders regions in. Converting a relative region needs the
		// size of the page, and knowing the size also lets us tell the user what
		// they should have used when the region misses the page completely.
		var renderCrop *requests.RenderPageCrop
		if parsedCrop != nil {
			pageSize, err := pdf.PdfiumInstance.GetPageSize(&requests.GetPageSize{
				Page: renderPages[0],
			})
			if err != nil {
				handleError(cmd, fmt.Errorf("could not get page size for page %s of PDF %s: %w\n", splitPages[0], args[0], newPdfiumError(err)), ExitCodePdfiumError)
				return
			}

			cropInPoints := parsedCrop.ToPoints(cropUnit, pageSize.Width, pageSize.Height, dpi)
			if cropInPoints.IsOutsidePage(pageSize.Width, pageSize.Height) {
				handleError(cmd, fmt.Errorf("the given crop falls completely outside of page %s, which is %.2f x %.2f points, the crop is %.2f x %.2f points at %.2f,%.2f, measured from the top-left corner of the page\n", splitPages[0], pageSize.Width, pageSize.Height, cropInPoints.Width, cropInPoints.Height, cropInPoints.X, cropInPoints.Y), ExitCodeInvalidArguments)
				return
			}

			renderCrop = &requests.RenderPageCrop{
				X:      cropInPoints.X,
				Y:      cropInPoints.Y,
				Width:  cropInPoints.Width,
				Height: cropInPoints.Height,
			}
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
								Crop:        renderCrop,
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
								Crop:        renderCrop,
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
