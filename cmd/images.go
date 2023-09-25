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

var (
	// Used for flags.
	jpegQuality int
)

func init() {
	addGenericPDFOptions(imagesCmd)
	addPagesOption("The pages or page to get images of", imagesCmd)
	imagesCmd.Flags().StringVarP(&fileType, "file-type", "", "jpeg", "The file type to render in, jpeg or png")
	imagesCmd.Flags().IntVarP(&jpegQuality, "jpeg-quality", "", 95, "Quality to use when file type is jpeg")

	rootCmd.AddCommand(imagesCmd)
}

var imagesCmd = &cobra.Command{
	Use:   "images [input] [output-folder]",
	Short: "Extract the images of a PDF",
	Long:  "Extract the images of a PDF and store them as file.\n[input] can either be a file path or - for stdin.\n[output-folder] can be either a folder or - for stdout. In the case of stdout, multiple files will be delimited by the value of the std-file-delimiter, with a newline before and after it.",
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

		parsedPageRange, _, err := pdf.NormalizePageRange(pageCount.PageCount, pageRange, false)
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

			objectCount, err := pdf.PdfiumInstance.FPDFPage_CountObjects(&requests.FPDFPage_CountObjects{
				Page: requests.Page{
					ByReference: &page.Page,
				},
			})

			if err != nil {
				closePageFunc()
				handleError(cmd, fmt.Errorf("could not get object count for page %d for PDF %s: %w\n", pageInt, args[0], newPdfiumError(err)), ExitCodePdfiumError)
				return
			}

			for i := 0; i < objectCount.Count; i++ {
				object, err := pdf.PdfiumInstance.FPDFPage_GetObject(&requests.FPDFPage_GetObject{
					Page: requests.Page{
						ByReference: &page.Page,
					},
					Index: i,
				})

				if err != nil {
					closePageFunc()
					handleError(cmd, fmt.Errorf("could not get object %d for page %d for PDF %s: %w\n", i, pageInt, args[0], newPdfiumError(err)), ExitCodePdfiumError)
					return
				}

				objectType, err := pdf.PdfiumInstance.FPDFPageObj_GetType(&requests.FPDFPageObj_GetType{
					PageObject: object.PageObject,
				})

				if err != nil {
					closePageFunc()
					handleError(cmd, fmt.Errorf("could not get object type for object %d for page %d for PDF %s: %w\n", i, pageInt, args[0], newPdfiumError(err)), ExitCodePdfiumError)
					return
				}

				if objectType.Type == enums.FPDF_PAGEOBJ_IMAGE {
					imageBitmap, err := pdf.PdfiumInstance.FPDFImageObj_GetRenderedBitmap(&requests.FPDFImageObj_GetRenderedBitmap{
						Document: document.Document,
						Page: requests.Page{
							ByReference: &page.Page,
						},
						ImageObject: object.PageObject,
					})

					if err != nil {
						closePageFunc()
						handleError(cmd, fmt.Errorf("could not get image for object %d for page %d for PDF %s: %w\n", i, pageInt, args[0], newPdfiumError(err)), ExitCodePdfiumError)
						return
					}

					closeBitmapFunc := func() {
						pdf.PdfiumInstance.FPDFBitmap_Destroy(&requests.FPDFBitmap_Destroy{
							Bitmap: imageBitmap.Bitmap,
						})
					}

					stride, err := pdf.PdfiumInstance.FPDFBitmap_GetStride(&requests.FPDFBitmap_GetStride{
						Bitmap: imageBitmap.Bitmap,
					})

					if err != nil {
						closePageFunc()
						closeBitmapFunc()
						handleError(cmd, fmt.Errorf("could not get image stride for object %d for page %d for PDF %s: %w\n", i, pageInt, args[0], newPdfiumError(err)), ExitCodePdfiumError)
						return
					}

					width, err := pdf.PdfiumInstance.FPDFBitmap_GetWidth(&requests.FPDFBitmap_GetWidth{
						Bitmap: imageBitmap.Bitmap,
					})

					if err != nil {
						closePageFunc()
						closeBitmapFunc()
						handleError(cmd, fmt.Errorf("could not get image width for object %d for page %d for PDF %s: %w\n", i, pageInt, args[0], newPdfiumError(err)), ExitCodePdfiumError)
						return
					}

					height, err := pdf.PdfiumInstance.FPDFBitmap_GetHeight(&requests.FPDFBitmap_GetHeight{
						Bitmap: imageBitmap.Bitmap,
					})

					if err != nil {
						closePageFunc()
						closeBitmapFunc()
						handleError(cmd, fmt.Errorf("could not get image height for object %d for page %d for PDF %s: %w\n", i, pageInt, args[0], newPdfiumError(err)), ExitCodePdfiumError)
						return
					}

					format, err := pdf.PdfiumInstance.FPDFBitmap_GetFormat(&requests.FPDFBitmap_GetFormat{
						Bitmap: imageBitmap.Bitmap,
					})

					if err != nil {
						closePageFunc()
						closeBitmapFunc()
						handleError(cmd, fmt.Errorf("could not get image format for object %d for page %d for PDF %s: %w\n", i, pageInt, args[0], newPdfiumError(err)), ExitCodePdfiumError)
						return
					}

					buffer, err := pdf.PdfiumInstance.FPDFBitmap_GetBuffer(&requests.FPDFBitmap_GetBuffer{
						Bitmap: imageBitmap.Bitmap,
					})

					if err != nil {
						closePageFunc()
						closeBitmapFunc()
						handleError(cmd, fmt.Errorf("could not get image buffer for object %d for page %d for PDF %s: %w\n", i, pageInt, args[0], newPdfiumError(err)), ExitCodePdfiumError)
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

					filePath := path.Join(args[1], fmt.Sprintf("page-%d-image-%d.%s", pageInt, i+1, ext))

					closeFunc := func() {}
					var outWriter io.Writer
					if args[1] != stdFilename {
						outFile, err := os.Create(filePath)
						if err != nil {
							closePageFunc()
							closeBitmapFunc()
							handleError(cmd, fmt.Errorf("could not create output file for object %d for page %d for PDF %s: %w\n", i, pageInt, args[0], err), ExitCodeInvalidOutput)
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
						cmd.Printf("Exported image %d from page %d into %s\n", i+1, pageInt, filePath)
					}

					imageCount++
				}
			}
		}
	},
}
