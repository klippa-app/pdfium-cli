package cmd

import (
	"fmt"
	"strings"

	"github.com/klippa-app/pdfium-cli/pdf"

	"github.com/klippa-app/go-pdfium/requests"
	"github.com/klippa-app/go-pdfium/responses"
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

		if err := validFile(args[0]); err != nil {
			return fmt.Errorf("could not open input file %s: %w\n", args[0], err)
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

		defer pdf.PdfiumInstance.FPDF_CloseDocument(&requests.FPDF_CloseDocument{Document: document.Document})

		fileVersion, err := pdf.PdfiumInstance.FPDF_GetFileVersion(&requests.FPDF_GetFileVersion{
			Document: document.Document,
		})
		if err != nil {
			cmd.PrintErr(fmt.Errorf("could not get version for PDF %s: %w\n", args[0], err))
			return
		}

		firstPartOfVersion := int(float64(fileVersion.FileVersion) / 10)
		secondPartOfVersion := fileVersion.FileVersion - (firstPartOfVersion * 10)
		cmd.Printf("PDF Version: %d.%d\n", firstPartOfVersion, secondPartOfVersion)

		metadata, err := pdf.PdfiumInstance.GetMetaData(&requests.GetMetaData{
			Document: document.Document,
		})
		if err == nil {
			// @todo: fix this when metadata has been properly fixed.
			if len(metadata.Tags) > 0 {
				cmd.Printf("Metadata:\n")
				for i := range metadata.Tags {
					cmd.Printf(" -  %s: %s\n", metadata.Tags[i].Tag, metadata.Tags[i].Value)
				}
			}
		}

		pageCount, err := pdf.PdfiumInstance.FPDF_GetPageCount(&requests.FPDF_GetPageCount{
			Document: document.Document,
		})
		if err != nil {
			cmd.PrintErr(fmt.Errorf("could not get page count for PDF %s: %w\n", args[0], err))
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
				cmd.PrintErr(fmt.Errorf("could not get page size for page %d of PDF %s: %w\n", i+1, args[0], err))
				return
			}

			label := ""
			pageLabel, err := pdf.PdfiumInstance.FPDF_GetPageLabel(&requests.FPDF_GetPageLabel{
				Document: document.Document,
				Page:     i,
			})
			if err == nil {
				label = pageLabel.Label
			}

			cmd.Printf(" - Page %d, size: %.2f x %.2f, label: %s\n", i+1, pageSize.Width, pageSize.Height, label)
		}

		permissions, err := pdf.PdfiumInstance.FPDF_GetDocPermissions(&requests.FPDF_GetDocPermissions{
			Document: document.Document,
		})
		if err != nil {
			cmd.PrintErr(fmt.Errorf("could not get permissions for PDF %s: %w\n", args[0], err))
			return
		}

		yesNo := func(in bool) string {
			if in {
				return "Allowed"
			}
			return "Forbidden"
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
			cmd.PrintErr(fmt.Errorf("could not get security handler revision for PDF %s: %w\n", args[0], err))
			return
		}

		cmd.Printf("Security Handler Revision: %d", securityHandlerRevision.SecurityHandlerRevision)
		if securityHandlerRevision.SecurityHandlerRevision == -1 {
			cmd.Printf(" (no protection)")
		}
		cmd.Printf("\n")

		signatureCount, err := pdf.PdfiumInstance.FPDF_GetSignatureCount(&requests.FPDF_GetSignatureCount{
			Document: document.Document,
		})
		if err != nil {
			// Signatures not enabled in this build.
			if isExperimentalError(err) {
				signatureCount = &responses.FPDF_GetSignatureCount{
					Count: 0,
				}
			} else {
				cmd.PrintErr(fmt.Errorf("could not get signature count for PDF %s: %w\n", args[0], err))
				return
			}
		}

		if signatureCount.Count > 0 {
			cmd.Printf("Signatures:\n")
			for i := 0; i < signatureCount.Count; i++ {
				signatureObj, err := pdf.PdfiumInstance.FPDF_GetSignatureObject(&requests.FPDF_GetSignatureObject{
					Document: document.Document,
					Index:    i,
				})
				if err != nil {
					cmd.PrintErr(fmt.Errorf("could not get signature object %d for PDF %s: %w\n", i, args[0], err))
					return
				}

				signatureTime, err := pdf.PdfiumInstance.FPDFSignatureObj_GetTime(&requests.FPDFSignatureObj_GetTime{
					Signature: signatureObj.Signature,
				})
				if err != nil {
					cmd.PrintErr(fmt.Errorf("could not get signature reason for signature object %d for PDF %s: %w\n", i, args[0], err))
					return
				}
				time := ""
				if signatureTime.Time != nil {
					time = *signatureTime.Time
				}

				signatureReason, err := pdf.PdfiumInstance.FPDFSignatureObj_GetReason(&requests.FPDFSignatureObj_GetReason{
					Signature: signatureObj.Signature,
				})
				if err != nil {
					cmd.PrintErr(fmt.Errorf("could not get signature reason for signature object %d for PDF %s: %w\n", i, args[0], err))
					return
				}

				reason := ""
				if signatureReason.Reason != nil {
					reason = *signatureReason.Reason
				}

				cmd.Printf(" - Signature %d, timestamp: %s, reason: %s\n", i+1, time, reason)
			}
		}

		attachments, err := pdf.PdfiumInstance.GetAttachments(&requests.GetAttachments{
			Document: document.Document,
		})
		if err != nil {
			cmd.PrintErr(fmt.Errorf("could not get signature count for PDF %s: %w\n", args[0], err))
			return
		}

		if len(attachments.Attachments) > 0 {
			cmd.Printf("Attachments:\n")
			for i := 0; i < len(attachments.Attachments); i++ {
				valueTexts := []string{}
				for valueI := i; valueI < len(attachments.Attachments[i].Values); valueI++ {
					valueTexts = append(valueTexts, fmt.Sprintf("%s (type %d): %s", attachments.Attachments[i].Values[valueI].Key, attachments.Attachments[i].Values[valueI].ValueType, attachments.Attachments[i].Values[valueI].StringValue))
				}
				cmd.Printf(" - Attachment %d, name: %s, values: %s\n", i+1, attachments.Attachments[i].Name, strings.Join(valueTexts, ", "))
			}
		}
	},
}
