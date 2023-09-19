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
	Long:  "Extract the attachments of a PDF and store them as file.\n[input] can either be a file path or - for stdin.\nThis extracts embedded thumbnails, it does not render a thumbnail of the page. Not all PDFs and pages have thumbnails. You can use the render command if you want to generate thumbnails.",
	Args: func(cmd *cobra.Command, args []string) error {
		if err := cobra.ExactArgs(2)(cmd, args); err != nil {
			return err
		}

		if err := validFile(args[0]); err != nil {
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

			imageBitmap, err := pdf.PdfiumInstance.FPDFPage_GetThumbnailAsBitmap(&requests.FPDFPage_GetThumbnailAsBitmap{
				Page: requests.Page{
					ByReference: &page.Page,
				},
			})

			if err != nil {
				closePageFunc()
				cmd.PrintErrf("could not get image for thumbnail of page %d for PDF %s: %w\n", pageInt, args[0], err)
				return
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
				cmd.PrintErrf("could not get image stride for thumbnail of page %d for PDF %s: %w\n", pageInt, args[0], err)
				return
			}

			width, err := pdf.PdfiumInstance.FPDFBitmap_GetWidth(&requests.FPDFBitmap_GetWidth{
				Bitmap: *imageBitmap.Bitmap,
			})

			if err != nil {
				closePageFunc()
				closeBitmapFunc()
				cmd.PrintErrf("could not get image width for thumbnail of page %d for PDF %s: %w\n", pageInt, args[0], err)
				return
			}

			height, err := pdf.PdfiumInstance.FPDFBitmap_GetHeight(&requests.FPDFBitmap_GetHeight{
				Bitmap: *imageBitmap.Bitmap,
			})

			if err != nil {
				closePageFunc()
				closeBitmapFunc()
				cmd.PrintErrf("could not get image height for thumbnail of page %d for PDF %s: %w\n", pageInt, args[0], err)
				return
			}

			format, err := pdf.PdfiumInstance.FPDFBitmap_GetFormat(&requests.FPDFBitmap_GetFormat{
				Bitmap: *imageBitmap.Bitmap,
			})

			if err != nil {
				closePageFunc()
				closeBitmapFunc()
				cmd.PrintErrf("could not get image format for thumbnail of page %d for PDF %s: %w\n", pageInt, args[0], err)
				return
			}

			buffer, err := pdf.PdfiumInstance.FPDFBitmap_GetBuffer(&requests.FPDFBitmap_GetBuffer{
				Bitmap: *imageBitmap.Bitmap,
			})

			if err != nil {
				closePageFunc()
				closeBitmapFunc()
				cmd.PrintErrf("could not get image buffer for thumbnail of page %d for PDF %s: %w\n", pageInt, args[0], err)
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
			outFile, err := os.Create(filePath)
			if err != nil {
				closePageFunc()
				closeBitmapFunc()
				cmd.PrintErrf("could not create output file for thumbnail of page %d for PDF %s: %w\n", pageInt, args[0], err)
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

			cmd.Printf("Exported thumbnail from page %d into %s\n", pageInt, filePath)
		}
	},
}
