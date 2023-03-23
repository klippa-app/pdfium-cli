package cmd

import (
	"fmt"
	"github.com/klippa-app/go-pdfium/enums"
	"image"
	"image/jpeg"
	"image/png"
	"os"
	"path"
	"strconv"
	"strings"

	"github.com/klippa-app/pdfium-cli/pdf"

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
	Long:  "Extract the images of a PDF and store them as file.",
	Args: func(cmd *cobra.Command, args []string) error {
		if err := cobra.ExactArgs(2)(cmd, args); err != nil {
			return err
		}

		if _, err := os.Stat(args[0]); err != nil {
			return fmt.Errorf("could not open input file %s: %w\n", args[0], err)
		}

		folderStat, err := os.Stat(args[1])
		if err != nil {
			return fmt.Errorf("could not open output folder %s: %w\n", args[1], err)
		}

		if !folderStat.IsDir() {
			return fmt.Errorf("output folder %s is not a folder\n", args[1])
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

		file, err := os.Open(args[0])
		if err != nil {
			cmd.PrintErrf("could not open input file %s: %w\n", args[0], err)
			return
		}
		defer file.Close()

		document, err := openFile(file)
		if err != nil {
			cmd.PrintErrf("could not open input file %s: %w\n", args[0], err)
			return
		}
		defer pdf.PdfiumInstance.FPDF_CloseDocument(&requests.FPDF_CloseDocument{Document: document.Document})

		pageCount, err := pdf.PdfiumInstance.FPDF_GetPageCount(&requests.FPDF_GetPageCount{
			Document: document.Document,
		})
		if err != nil {
			cmd.PrintErrf("could not get page count for PDF %s: %w\n", args[0], err)
			return
		}

		pageRange := "first-last"
		if pages != "" {
			pageRange = pages
		}

		parsedPageRange, _, err := pdf.NormalizePageRange(pageCount.PageCount, pageRange, false)
		if err != nil {
			cmd.PrintErrf("invalid page range '%s': %s\n", pageRange, err)
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
				cmd.PrintErrf("could not load page for page %d for PDF %s: %w\n", pageInt, args[0], err)
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
				cmd.PrintErrf("could not get object count for page %d for PDF %s: %w\n", pageInt, args[0], err)
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
					cmd.PrintErrf("could not get object %d for page %d for PDF %s: %w\n", i, pageInt, args[0], err)
					return
				}

				objectType, err := pdf.PdfiumInstance.FPDFPageObj_GetType(&requests.FPDFPageObj_GetType{
					PageObject: object.PageObject,
				})

				if err != nil {
					closePageFunc()
					cmd.PrintErrf("could not get object type for object %d for page %d for PDF %s: %w\n", i, pageInt, args[0], err)
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
						cmd.PrintErrf("could not get image for object %d for page %d for PDF %s: %w\n", i, pageInt, args[0], err)
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
						cmd.PrintErrf("could not get image stride for object %d for page %d for PDF %s: %w\n", i, pageInt, args[0], err)
						return
					}

					width, err := pdf.PdfiumInstance.FPDFBitmap_GetWidth(&requests.FPDFBitmap_GetWidth{
						Bitmap: imageBitmap.Bitmap,
					})

					if err != nil {
						closePageFunc()
						closeBitmapFunc()
						cmd.PrintErrf("could not get image width for object %d for page %d for PDF %s: %w\n", i, pageInt, args[0], err)
						return
					}

					height, err := pdf.PdfiumInstance.FPDFBitmap_GetHeight(&requests.FPDFBitmap_GetHeight{
						Bitmap: imageBitmap.Bitmap,
					})

					if err != nil {
						closePageFunc()
						closeBitmapFunc()
						cmd.PrintErrf("could not get image height for object %d for page %d for PDF %s: %w\n", i, pageInt, args[0], err)
						return
					}

					format, err := pdf.PdfiumInstance.FPDFBitmap_GetFormat(&requests.FPDFBitmap_GetFormat{
						Bitmap: imageBitmap.Bitmap,
					})

					if err != nil {
						closePageFunc()
						closeBitmapFunc()
						cmd.PrintErrf("could not get image format for object %d for page %d for PDF %s: %w\n", i, pageInt, args[0], err)
						return
					}

					buffer, err := pdf.PdfiumInstance.FPDFBitmap_GetBuffer(&requests.FPDFBitmap_GetBuffer{
						Bitmap: imageBitmap.Bitmap,
					})

					if err != nil {
						closePageFunc()
						closeBitmapFunc()
						cmd.PrintErrf("could not get image buffer for object %d for page %d for PDF %s: %w\n", i, pageInt, args[0], err)
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
					outFile, err := os.Create(filePath)
					if err != nil {
						closePageFunc()
						closeBitmapFunc()
						cmd.PrintErrf("could not create output file for object %d for page %d for PDF %s: %w\n", i, pageInt, args[0], err)
						return
					}

					if fileType == "png" {
						err = png.Encode(outFile, img)
						if err != nil {
							closePageFunc()
							closeBitmapFunc()
							outFile.Close()
							return
						}
					} else {
						var opt jpeg.Options
						opt.Quality = jpegQuality

						err = jpeg.Encode(outFile, img, &opt)
						if err != nil {
							closePageFunc()
							closeBitmapFunc()
							outFile.Close()
							return
						}
					}

					outFile.Close()
					closeBitmapFunc()

					cmd.Printf("Exported image %d from page %d into %s\n", i+1, pageInt, filePath)
				}
			}
		}
	},
}
