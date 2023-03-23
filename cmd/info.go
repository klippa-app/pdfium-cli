package cmd

import (
	"fmt"
	"os"

	"github.com/klippa-app/pdfium-cli/pdf"

	"github.com/klippa-app/go-pdfium/requests"
	"github.com/spf13/cobra"
)

func init() {
	addGenericPDFOptions(infoCmd)
	rootCmd.AddCommand(infoCmd)
}

var infoCmd = &cobra.Command{
	Use:   "info [input]",
	Short: "Get the information of a PDF",
	Long:  "Get the information of a PDF and its pages, like metadata and page size.",
	Args: func(cmd *cobra.Command, args []string) error {
		if err := cobra.ExactArgs(1)(cmd, args); err != nil {
			return err
		}

		if _, err := os.Stat(args[0]); err != nil {
			return fmt.Errorf("could not open input file %s: %w\n", args[0], err)
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

		fileVersion, err := pdf.PdfiumInstance.FPDF_GetFileVersion(&requests.FPDF_GetFileVersion{
			Document: document.Document,
		})
		if err != nil {
			cmd.PrintErrf("could not get version for PDF %s: %w\n", args[0], err)
			return
		}

		firstPartOfVersion := int(float64(fileVersion.FileVersion) / 10)
		secondPartOfVersion := fileVersion.FileVersion - (firstPartOfVersion * 10)
		cmd.Printf("PDF Version: %d.%d\n", firstPartOfVersion, secondPartOfVersion)

		metadata, err := pdf.PdfiumInstance.GetMetaData(&requests.GetMetaData{
			Document: document.Document,
		})
		if err != nil {
			cmd.PrintErrf("could not get metadata for PDF %s: %w\n", args[0], err)
			return
		}

		if len(metadata.Tags) > 0 {
			cmd.Printf("Metadata:\n")
			for i := range metadata.Tags {
				cmd.Printf(" -  %s: %s\n", metadata.Tags[i].Tag, metadata.Tags[i].Value)
			}
		}

		pageCount, err := pdf.PdfiumInstance.FPDF_GetPageCount(&requests.FPDF_GetPageCount{
			Document: document.Document,
		})
		if err != nil {
			cmd.PrintErrf("could not get page count for PDF %s: %w\n", args[0], err)
			return
		}

		cmd.Printf("Page count: %d\nPage size (in points (WxH), one point is 1/72 inch (around 0.3528 mm)):\n", pageCount.PageCount)

		for i := 0; i < pageCount.PageCount; i++ {
			pageSize, err := pdf.PdfiumInstance.GetPageSize(&requests.GetPageSize{
				Page: requests.Page{
					ByIndex: &requests.PageByIndex{
						Document: document.Document,
						Index:    i,
					},
				},
			})
			if err != nil {
				cmd.PrintErrf("could not get page size for page %d of PDF %s: %w\n", i+1, args[0], err)
				return
			}

			cmd.Printf(" - Page %d: %.2f x %.2f\n", i+1, pageSize.Width, pageSize.Height)
		}

		permissions, err := pdf.PdfiumInstance.FPDF_GetDocPermissions(&requests.FPDF_GetDocPermissions{
			Document: document.Document,
		})
		if err != nil {
			cmd.PrintErrf("could not get permissions for PDF %s: %w\n", args[0], err)
			return
		}

		yesNo := func(in bool) string {
			if in {
				return "Yes"
			}
			return "No"
		}

		cmd.Printf("Permissions:\n")
		cmd.Printf(" - Print Document: %s\n", yesNo(permissions.PrintDocument))
		cmd.Printf(" - Modify Contents: %s\n", yesNo(permissions.ModifyContents))
		cmd.Printf(" - Copy Or Extract Text: %s\n", yesNo(permissions.CopyOrExtractText))
		cmd.Printf(" - Add Or Modify Text Annotations: %s\n", yesNo(permissions.AddOrModifyTextAnnotations))
		cmd.Printf(" - Fill In Interactive Form Fields: %s\n", yesNo(permissions.FillInInteractiveFormFields))
		cmd.Printf(" - Create Or Modify Interactive Form Fields: %s\n", yesNo(permissions.CreateOrModifyInteractiveFormFields))
		cmd.Printf(" - Fill In Existing Interactive Form Fields: %s\n", yesNo(permissions.FillInExistingInteractiveFormFields))
		cmd.Printf(" - Extract Text And Graphics: %s\n", yesNo(permissions.ExtractTextAndGraphics))
		cmd.Printf(" - Assemble Document: %s\n", yesNo(permissions.AssembleDocument))
		cmd.Printf(" - Print Document As Faithful Digital Copy: %s\n", yesNo(permissions.PrintDocumentAsFaithfulDigitalCopy))

		securityHandlerRevision, err := pdf.PdfiumInstance.FPDF_GetSecurityHandlerRevision(&requests.FPDF_GetSecurityHandlerRevision{
			Document: document.Document,
		})
		if err != nil {
			cmd.PrintErrf("could not get security handler revision for PDF %s: %w\n", args[0], err)
			return
		}

		cmd.Printf("Security Handler Revision: %d", securityHandlerRevision.SecurityHandlerRevision)
		if securityHandlerRevision.SecurityHandlerRevision == -1 {
			cmd.Printf(" (no protection)")
		}
		cmd.Printf("\n")
	},
}
