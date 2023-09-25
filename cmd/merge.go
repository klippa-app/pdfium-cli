package cmd

import (
	"fmt"
	"io"
	"os"

	"github.com/klippa-app/go-pdfium/requests"

	"github.com/klippa-app/pdfium-cli/pdf"
	"github.com/spf13/cobra"
)

func init() {
	addGenericPDFOptions(mergeCmd)
	rootCmd.AddCommand(mergeCmd)
}

var mergeCmd = &cobra.Command{
	Use:   "merge [input] [input] ([input]...) [output]",
	Short: "Merge multiple PDFs into a single PDF",
	Long:  "Merge multiple PDFs into a single PDF.\n[output] can either be a file path or - for stdout.",
	Args: func(cmd *cobra.Command, args []string) error {
		if args[0] == stdFilename {
			if err := cobra.MinimumNArgs(2)(cmd, args); err != nil {
				return err
			}
		} else {
			if err := cobra.MinimumNArgs(3)(cmd, args); err != nil {
				return err
			}

			for i := 0; i < len(args)-1; i++ {
				if _, err := os.Stat(args[i]); err != nil {
					return fmt.Errorf("could not open input file %s: %w", args[0], err)
				}
			}
		}

		return nil
	},
	Run: func(cmd *cobra.Command, args []string) {
		err := pdf.LoadPdfium()
		if err != nil {
			cmd.PrintErr(fmt.Errorf("could not load pdfium: %w", err))
			return
		}
		defer pdf.ClosePdfium()

		newDocument, err := pdf.PdfiumInstance.FPDF_CreateNewDocument(&requests.FPDF_CreateNewDocument{})
		if err != nil {
			cmd.PrintErr(fmt.Errorf("could not create new document: %w", err))
			return
		}

		mergedPageCount := 0
		i := 0
		for true {
			var filename string
			if args[0] == stdFilename {
				filename = stdFilename
			} else {
				// Reached last file.
				if i == len(args)-1 {
					break
				}
				filename = args[i]
			}

			document, closeFile, err := openFile(filename)
			if err != nil {
				// Reached last stdin file.
				if err == stdinNoMoreFiles {
					break
				}
				cmd.PrintErr(fmt.Errorf("could not open input file %s: %w", args[i], err))
				return
			}

			closeFunc := func() {
				closeFile()
			}

			pageCount, err := pdf.PdfiumInstance.FPDF_GetPageCount(&requests.FPDF_GetPageCount{
				Document: document.Document,
			})
			if err != nil {
				closeFunc()
				cmd.PrintErr(fmt.Errorf("could not get page ranges for file %s: %w", args[i], err))
				return
			}

			pageRange, calculatedPageCount, err := pdf.NormalizePageRange(pageCount.PageCount, "first-last")
			if err != nil {
				closeFunc()
				cmd.PrintErr(fmt.Errorf("could not calculate page range for file %s: %w", args[i], err))
				return
			}

			_, err = pdf.PdfiumInstance.FPDF_ImportPages(&requests.FPDF_ImportPages{
				Source:      document.Document,
				Destination: newDocument.Document,
				PageRange:   pageRange,
				Index:       mergedPageCount,
			})
			if err != nil {
				closeFunc()
				cmd.PrintErr(fmt.Errorf("could not import pages for file %s: %w", args[i], err))
				return
			}

			mergedPageCount += *calculatedPageCount

			_, err = pdf.PdfiumInstance.FPDF_CloseDocument(&requests.FPDF_CloseDocument{
				Document: document.Document,
			})
			if err != nil {
				closeFunc()
				cmd.PrintErr(fmt.Errorf("could not close document for file %s: %w", args[i], err))
				return
			}

			closeFunc()
			i++
		}

		var fileWriter io.Writer
		if args[len(args)-1] == stdFilename {
			fileWriter = os.Stdout
		} else {
			createdFile, err := os.Create(args[len(args)-1])
			if err != nil {
				cmd.PrintErr(fmt.Errorf("could not save document: %w", err))
				return
			}

			defer createdFile.Close()
			fileWriter = createdFile
		}

		_, err = pdf.PdfiumInstance.FPDF_SaveAsCopy(&requests.FPDF_SaveAsCopy{
			Document:   newDocument.Document,
			FileWriter: fileWriter,
		})
		if err != nil {
			cmd.PrintErr(fmt.Errorf("could not save new document %s: %w", err))
			return
		}

		_, err = pdf.PdfiumInstance.FPDF_CloseDocument(&requests.FPDF_CloseDocument{
			Document: newDocument.Document,
		})
		if err != nil {
			cmd.PrintErr(fmt.Errorf("could not save new document %s: %w", err))
			return
		}
	},
}
