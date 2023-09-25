package cmd

import (
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"os"
	"path"
	"strconv"
	"strings"

	"github.com/klippa-app/pdfium-cli/pdf"

	"github.com/klippa-app/go-pdfium/enums"
	"github.com/klippa-app/go-pdfium/requests"
	"github.com/spf13/cobra"
)

func init() {
	addGenericPDFOptions(thumbnailsCmd)
	addPagesOption("The pages or page to get thumbnails of", thumbnailsCmd)
	thumbnailsCmd.Flags().StringVarP(&fileType, "file-type", "", "jpeg", "The file type to render in, jpeg or png")
	thumbnailsCmd.Flags().IntVarP(&jpegQuality, "jpeg-quality", "", 95, "Quality to use when file type is jpeg")

	rootCmd.AddCommand(thumbnailsCmd)
}

var thumbnailsCmd = &cobra.Command{
	Use:   "thumbnails [input] [output-folder]",
	Short: "Extract the attachments of a PDF",
	Long:  "Extract the attachments of a PDF and store them as file.\n[input] can either be a file path or - for stdin.\nThis extracts embedded thumbnails, it does not render a thumbnail of the page. Not all PDFs and pages have thumbnails. You can use the render command if you want to generate thumbnails.[output-folder] can be either a folder or - for stdout. In the case of stdout, multiple files will be delimited by the value of the std-file-delimiter, with a newline before and after it.",
	Args: func(cmd *cobra.Command, args []string) error {
		if err := cobra.ExactArgs(2)(cmd, args); err != nil {
			return newExitCodeError(err, ExitCodeInvalidArguments)
		}

		if err := validFile(args[0]); err != nil {
			return fmt.Errorf("could not open input file %s: %w\n", args[0], newExitCodeError(err, ExitCodeInvalidInput))
		}

		if args[1] != stdFilename {
			folderStat, err := os.Stat(args[1])
			if err != nil {
				return fmt.Errorf("could not open output folder %s: %w\n", args[1], newExitCodeError(err, ExitCodeInvalidOutput))
			}

			if !folderStat.IsDir() {
				return newExitCodeError(fmt.Errorf("output folder %s is not a folder\n", args[1]), ExitCodeInvalidOutput)
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

		parsedPageRange, _, err := pdf.NormalizePageRange(pageCount.PageCount, pageRange)
		if err != nil {
			handleError(cmd, fmt.Errorf("invalid page range '%s': %w\n", pageRange, err), ExitCodeInvalidPageRange)
			return
		}

		pages := strings.Split(*parsedPageRange, ",")
		imageCount := 0
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

			imageBitmap, err := pdf.PdfiumInstance.FPDFPage_GetThumbnailAsBitmap(&requests.FPDFPage_GetThumbnailAsBitmap{
				Page: requests.Page{
					ByReference: &page.Page,
				},
			})

			if err != nil {
				closePageFunc()
				// Thumbnails not enabled in this build.
				if isExperimentalError(err) {
					handleError(cmd, fmt.Errorf("Thumbnail support is not enabled in your build, build with the build tag pdfium_experimental to enable!\n"), ExitCodeExperimental)
					return
				} else {
					handleError(cmd, fmt.Errorf("could not get image for thumbnail of page %d for PDF %s: %w\n", pageInt, args[0], newPdfiumError(err)), ExitCodePdfiumError)
					return
				}
			}

			// No bitmap found.
			if imageBitmap.Bitmap == nil {
				continue
			}

			closeBitmapFunc := func() {
				pdf.PdfiumInstance.FPDFBitmap_Destroy(&requests.FPDFBitmap_Destroy{
					Bitmap: *imageBitmap.Bitmap,
				})
			}

			stride, err := pdf.PdfiumInstance.FPDFBitmap_GetStride(&requests.FPDFBitmap_GetStride{
				Bitmap: *imageBitmap.Bitmap,
			})

			if err != nil {
				closePageFunc()
				closeBitmapFunc()
				handleError(cmd, fmt.Errorf("could not get image stride for thumbnail of page %d for PDF %s: %w\n", pageInt, args[0], newPdfiumError(err)), ExitCodePdfiumError)
				return
			}

			width, err := pdf.PdfiumInstance.FPDFBitmap_GetWidth(&requests.FPDFBitmap_GetWidth{
				Bitmap: *imageBitmap.Bitmap,
			})

			if err != nil {
				closePageFunc()
				closeBitmapFunc()
				handleError(cmd, fmt.Errorf("could not get image width for thumbnail of page %d for PDF %s: %w\n", pageInt, args[0], newPdfiumError(err)), ExitCodePdfiumError)
				return
			}

			height, err := pdf.PdfiumInstance.FPDFBitmap_GetHeight(&requests.FPDFBitmap_GetHeight{
				Bitmap: *imageBitmap.Bitmap,
			})

			if err != nil {
				closePageFunc()
				closeBitmapFunc()
				handleError(cmd, fmt.Errorf("could not get image height for thumbnail of page %d for PDF %s: %w\n", pageInt, args[0], newPdfiumError(err)), ExitCodePdfiumError)
				return
			}

			format, err := pdf.PdfiumInstance.FPDFBitmap_GetFormat(&requests.FPDFBitmap_GetFormat{
				Bitmap: *imageBitmap.Bitmap,
			})

			if err != nil {
				closePageFunc()
				closeBitmapFunc()
				handleError(cmd, fmt.Errorf("could not get image format for thumbnail of page %d for PDF %s: %w\n", pageInt, args[0], newPdfiumError(err)), ExitCodePdfiumError)
				return
			}

			buffer, err := pdf.PdfiumInstance.FPDFBitmap_GetBuffer(&requests.FPDFBitmap_GetBuffer{
				Bitmap: *imageBitmap.Bitmap,
			})

			if err != nil {
				closePageFunc()
				closeBitmapFunc()
				handleError(cmd, fmt.Errorf("could not get image buffer for thumbnail of page %d for PDF %s: %w\n", pageInt, args[0], newPdfiumError(err)), ExitCodePdfiumError)
				return
			}

			var img image.Image
			if format.Format == enums.FPDF_BITMAP_FORMAT_BGRA {
				img = &BGRA{
					Pix:    buffer.Buffer,
					Stride: stride.Stride,
					Rect:   image.Rect(0, 0, width.Width, height.Height),
				}
			} else if format.Format == enums.FPDF_BITMAP_FORMAT_BGRX {
				img = &BGRX{
					Pix:    buffer.Buffer,
					Stride: stride.Stride,
					Rect:   image.Rect(0, 0, width.Width, height.Height),
				}
			} else if format.Format == enums.FPDF_BITMAP_FORMAT_BGR {
				img = &BGR{
					Pix:    buffer.Buffer,
					Stride: stride.Stride,
					Rect:   image.Rect(0, 0, width.Width, height.Height),
				}
			} else if format.Format == enums.FPDF_BITMAP_FORMAT_GRAY {
				img = &image.Gray{
					Pix:    buffer.Buffer,
					Stride: stride.Stride,
					Rect:   image.Rect(0, 0, width.Width, height.Height),
				}
			}

			ext := "jpg"
			if fileType == "png" {
				ext = "png"
			}

			filePath := path.Join(args[1], fmt.Sprintf("thumbnail-page-%d.%s", pageInt, ext))

			closeFunc := func() {}
			var outWriter io.Writer
			if args[1] != stdFilename {
				outFile, err := os.Create(filePath)
				if err != nil {
					closePageFunc()
					closeBitmapFunc()
					handleError(cmd, fmt.Errorf("could not create output file for thumbnail of page %d for PDF %s: %w\n", pageInt, args[0], err), ExitCodeInvalidOutput)
					return
				}
				outWriter = outFile
				closeFunc = func() {
					outFile.Close()
				}
			} else {
				if imageCount > 0 {
					os.Stdout.WriteString("\n")
					os.Stdout.WriteString(stdFileDelimiter)
					os.Stdout.WriteString("\n")
				}
				outWriter = os.Stdout
			}

			if fileType == "png" {
				err = png.Encode(outWriter, img)
				if err != nil {
					closePageFunc()
					closeBitmapFunc()
					closeFunc()
					return
				}
			} else {
				var opt jpeg.Options
				opt.Quality = jpegQuality

				err = jpeg.Encode(outWriter, img, &opt)
				if err != nil {
					closePageFunc()
					closeBitmapFunc()
					closeFunc()
					return
				}
			}

			closeFunc()
			closeBitmapFunc()

			if args[1] != stdFilename {
				cmd.Printf("Exported thumbnail from page %d into %s\n", pageInt, filePath)
			}

			imageCount++
		}
	},
}
