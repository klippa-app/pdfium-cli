package cmd

import (
	"encoding/json"
	"fmt"
	"github.com/klippa-app/go-pdfium/responses"
	"strconv"
	"strings"

	"github.com/klippa-app/pdfium-cli/pdf"

	"github.com/klippa-app/go-pdfium/requests"
	"github.com/spf13/cobra"
)

var (
	// Used for flags.
	outputType                          string
	textPageHeader                      bool
	jsonOutputDetails                   string
	jsonFullCollectFontInformation      bool
	jsonFullCollectPixelPositionsDPI    int
	jsonFullCollectPixelPositionsWidth  int
	jsonFullCollectPixelPositionsHeight int
)

func init() {
	addGenericPDFOptions(textCmd)
	addPagesOption("The pages or page to get text of", textCmd)

	textCmd.Flags().StringVarP(&outputType, "output-type", "", "text", "The file type to output, text or json")
	textCmd.Flags().BoolVarP(&textPageHeader, "text-page-header", "", true, "Whether to add page headers to the text to indicate the page number.")
	textCmd.Flags().StringVarP(&jsonOutputDetails, "json-output-details", "", "compact", "The level of details in the output when using JSON as output format, compact or full. compact will only give you the text per page, full will give you coordinates per character and rectangles that have characters together.")
	textCmd.Flags().BoolVarP(&jsonFullCollectFontInformation, "json-full-collect-font-information", "", false, "Whether to collection font information. Only available in output format json and output details full")
	textCmd.Flags().IntVarP(&jsonFullCollectPixelPositionsDPI, "json-full-pixel-positions-dpi", "", 0, "DPI you used when rendering to calculate pixels positions. Only available in output format json and output details full")
	textCmd.Flags().IntVarP(&jsonFullCollectPixelPositionsWidth, "json-full-pixel-positions-width", "", 0, "Width you used when rendering to calculate pixel positions. Only available in output format json and output details full")
	textCmd.Flags().IntVarP(&jsonFullCollectPixelPositionsHeight, "json-full-pixel-positions-height", "", 0, "Height you used when rendering to calculate pixel positions. Only available in output format json and output details full")

	rootCmd.AddCommand(textCmd)
}

var textCmd = &cobra.Command{
	Use:   "text [input]",
	Short: "Get the text of a PDF",
	Long:  "Get the text of a PDF in text or json.\n[input] can either be a file path or - for stdin.",
	Args: func(cmd *cobra.Command, args []string) error {
		if err := cobra.ExactArgs(1)(cmd, args); err != nil {
			return err
		}

		if err := validFile(args[0]); err != nil {
			return fmt.Errorf("could not open input file %s: %w", args[0], err)
		}

		return nil
	},
	Run: func(cmd *cobra.Command, args []string) {
		err := pdf.LoadPdfium()
		if err != nil {
			cmd.PrintErr(fmt.Errorf("could not load pdfium: %w\n", err))
			return
		}
		defer pdf.ClosePdfium()

		document, closeFile, err := openFile(args[0])
		if err != nil {
			cmd.PrintErr(fmt.Errorf("could not open input file %s: %w\n", args[0], err))
			return
		}

		defer closeFile()

		pageCount, err := pdf.PdfiumInstance.FPDF_GetPageCount(&requests.FPDF_GetPageCount{
			Document: document.Document,
		})
		if err != nil {
			cmd.PrintErr(fmt.Errorf("could not get page count for PDF %s: %w\n", args[0], err))
			return
		}

		pageRange := "first-last"
		if pages != "" {
			pageRange = pages
		}

		parsedPageRange, _, err := pdf.NormalizePageRange(pageCount.PageCount, pageRange, false)
		if err != nil {
			cmd.PrintErr(fmt.Errorf("invalid page range '%s': %s\n", pageRange, err))
			return
		}

		textPages := []requests.Page{}
		splitPages := strings.Split(*parsedPageRange, ",")
		for _, page := range splitPages {
			pageInt, _ := strconv.Atoi(page)
			textPages = append(textPages, requests.Page{
				ByIndex: &requests.PageByIndex{
					Document: document.Document,
					Index:    pageInt - 1, // pdfium is 0-index based
				},
			})
		}

		var pixelPositions requests.GetPageTextStructuredPixelPositions
		if jsonFullCollectPixelPositionsDPI > 0 || jsonFullCollectPixelPositionsWidth > 0 || jsonFullCollectPixelPositionsHeight > 0 {
			pixelPositions = requests.GetPageTextStructuredPixelPositions{
				Document:  document.Document,
				Calculate: true,
				DPI:       jsonFullCollectPixelPositionsDPI,
				Width:     jsonFullCollectPixelPositionsWidth,
				Height:    jsonFullCollectPixelPositionsHeight,
			}
		}

		if outputType == "json" && jsonOutputDetails == "full" {
			jsonPageText := []*responses.GetPageTextStructured{}
			for i := range textPages {
				pageText, err := pdf.PdfiumInstance.GetPageTextStructured(&requests.GetPageTextStructured{
					Page:                   textPages[i],
					CollectFontInformation: jsonFullCollectFontInformation,
					PixelPositions:         pixelPositions,
				})
				if err != nil {
					cmd.PrintErr(fmt.Errorf("could not get page size for page %d of PDF %s: %w\n", i+1, args[0], err))
					return
				}

				jsonPageText = append(jsonPageText, pageText)
			}

			outputJson, _ := json.MarshalIndent(jsonPageText, "", "  ")
			cmd.Println(string(outputJson))
		} else {
			jsonPageText := []*responses.GetPageText{}
			for i := range textPages {
				pageText, err := pdf.PdfiumInstance.GetPageText(&requests.GetPageText{
					Page: textPages[i],
				})
				if err != nil {
					cmd.PrintErr(fmt.Errorf("could not get page size for page %d of PDF %s: %w\n", i+1, args[0], err))
					return
				}

				if outputType == "json" {
					jsonPageText = append(jsonPageText, pageText)
				} else {
					// Inject newline after text block.
					if i > 0 {
						cmd.Printf("\n")
					}
					if textPageHeader {
						cmd.Printf("Page %d\n", textPages[i].ByIndex.Index+1)
					}
					cmd.Printf("%s\n", pageText.Text)
				}
			}

			if outputType == "json" {
				outputJson, _ := json.MarshalIndent(jsonPageText, "", "  ")
				cmd.Println(string(outputJson))
			}
		}
	},
}
