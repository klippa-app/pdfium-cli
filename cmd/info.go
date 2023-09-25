package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/klippa-app/pdfium-cli/pdf"

	"github.com/klippa-app/go-pdfium/requests"
	"github.com/klippa-app/go-pdfium/responses"
	"github.com/spf13/cobra"
)

func init() {
	addGenericPDFOptions(infoCmd)
	infoCmd.Flags().StringVarP(&outputType, "output-type", "", "text", "The file type to output, text or json")
	rootCmd.AddCommand(infoCmd)
}

var infoCmd = &cobra.Command{
	Use:   "info [input] [output]",
	Short: "Get the information of a PDF",
	Long:  "Get the information of a PDF and its pages, like metadata and page size.\n[input] can either be a file path or - for stdin.\n[output] can either be a file path or - for stdout (default).",
	Args: func(cmd *cobra.Command, args []string) error {
		if err := cobra.MinimumNArgs(1)(cmd, args); err != nil {
			return newExitCodeError(err, ExitCodeInvalidArguments)
		}

		if err := validFile(args[0]); err != nil {
			return fmt.Errorf("could not open input file %s: %w\n", args[0], newExitCodeError(err, ExitCodeInvalidInput))
		}

		return nil
	},
	Run: func(cmd *cobra.Command, args []string) {
		// Second argument is the output file.
		if len(args) > 1 && args[1] != stdFilename {
			createdFile, err := os.Create(args[1])
			if err != nil {
				handleError(cmd, fmt.Errorf("could not create file: %w", err), ExitCodeInvalidOutput)
				return
			}

			defer createdFile.Close()
			cmd.SetOut(createdFile)
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

		defer pdf.PdfiumInstance.FPDF_CloseDocument(&requests.FPDF_CloseDocument{Document: document.Document})

		fileVersion, err := pdf.PdfiumInstance.FPDF_GetFileVersion(&requests.FPDF_GetFileVersion{
			Document: document.Document,
		})
		if err != nil {
			handleError(cmd, fmt.Errorf("could not get version for PDF %s: %w\n", args[0], newPdfiumError(err)), ExitCodePdfiumError)
			return
		}

		type pdfInfoMetadata struct {
			Tag   string
			Value string
		}

		type pdfPage struct {
			Number int
			Width  float64
			Height float64
			Label  string
		}

		type pdfSignature struct {
			Time   *string
			Reason *string
		}

		type pdfInfoStruct struct {
			VersionNumber           int
			Version                 string
			Metadata                []pdfInfoMetadata
			PageCount               int
			Pages                   []pdfPage
			Permissions             *responses.FPDF_GetDocPermissions
			SecurityHandlerRevision int
			Signatures              []pdfSignature
			Attachments             []responses.Attachment
		}

		pdfInfo := &pdfInfoStruct{
			Metadata:    []pdfInfoMetadata{},
			Pages:       []pdfPage{},
			Signatures:  []pdfSignature{},
			Attachments: []responses.Attachment{},
		}

		firstPartOfVersion := int(float64(fileVersion.FileVersion) / 10)
		secondPartOfVersion := fileVersion.FileVersion - (firstPartOfVersion * 10)

		pdfInfo.VersionNumber = fileVersion.FileVersion
		pdfInfo.Version = fmt.Sprintf("%d.%d", firstPartOfVersion, secondPartOfVersion)

		metadata, err := pdf.PdfiumInstance.GetMetaData(&requests.GetMetaData{
			Document: document.Document,
		})
		if err == nil {
			// @todo: fix this when metadata has been properly fixed.
			if len(metadata.Tags) > 0 {
				for i := range metadata.Tags {
					pdfInfo.Metadata = append(pdfInfo.Metadata, pdfInfoMetadata{
						Tag:   metadata.Tags[i].Tag,
						Value: metadata.Tags[i].Value,
					})
				}
			}
		}

		pageCount, err := pdf.PdfiumInstance.FPDF_GetPageCount(&requests.FPDF_GetPageCount{
			Document: document.Document,
		})
		if err != nil {
			handleError(cmd, fmt.Errorf("could not get page count for PDF %s: %w\n", args[0], newPdfiumError(err)), ExitCodePdfiumError)
			return
		}

		pdfInfo.PageCount = pageCount.PageCount

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
				handleError(cmd, fmt.Errorf("could not get page size for page %d of PDF %s: %w\n", i+1, args[0], newPdfiumError(err)), ExitCodePdfiumError)
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

			pdfInfo.Pages = append(pdfInfo.Pages, pdfPage{
				Number: i + 1,
				Width:  pageSize.Width,
				Height: pageSize.Height,
				Label:  label,
			})
		}

		permissions, err := pdf.PdfiumInstance.FPDF_GetDocPermissions(&requests.FPDF_GetDocPermissions{
			Document: document.Document,
		})
		if err != nil {
			handleError(cmd, fmt.Errorf("could not get permissions for PDF %s: %w\n", args[0], newPdfiumError(err)), ExitCodePdfiumError)
			return
		}

		pdfInfo.Permissions = permissions

		securityHandlerRevision, err := pdf.PdfiumInstance.FPDF_GetSecurityHandlerRevision(&requests.FPDF_GetSecurityHandlerRevision{
			Document: document.Document,
		})
		if err != nil {
			handleError(cmd, fmt.Errorf("could not get security handler revision for PDF %s: %w\n", args[0], newPdfiumError(err)), ExitCodePdfiumError)
			return
		}

		pdfInfo.SecurityHandlerRevision = securityHandlerRevision.SecurityHandlerRevision

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
				handleError(cmd, fmt.Errorf("could not get signature count for PDF %s: %w\n", args[0], newPdfiumError(err)), ExitCodePdfiumError)
				return
			}
		}

		if signatureCount.Count > 0 {
			for i := 0; i < signatureCount.Count; i++ {
				signatureObj, err := pdf.PdfiumInstance.FPDF_GetSignatureObject(&requests.FPDF_GetSignatureObject{
					Document: document.Document,
					Index:    i,
				})
				if err != nil {
					handleError(cmd, fmt.Errorf("could not get signature object %d for PDF %s: %w\n", i, args[0], newPdfiumError(err)), ExitCodePdfiumError)
					return
				}

				signatureTime, err := pdf.PdfiumInstance.FPDFSignatureObj_GetTime(&requests.FPDFSignatureObj_GetTime{
					Signature: signatureObj.Signature,
				})
				if err != nil {
					handleError(cmd, fmt.Errorf("could not get signature reason for signature object %d for PDF %s: %w\n", i, args[0], newPdfiumError(err)), ExitCodePdfiumError)
					return
				}

				signatureReason, err := pdf.PdfiumInstance.FPDFSignatureObj_GetReason(&requests.FPDFSignatureObj_GetReason{
					Signature: signatureObj.Signature,
				})
				if err != nil {
					handleError(cmd, fmt.Errorf("could not get signature reason for signature object %d for PDF %s: %w\n", i, args[0], newPdfiumError(err)), ExitCodePdfiumError)
					return
				}

				pdfInfo.Signatures = append(pdfInfo.Signatures, pdfSignature{
					Time:   signatureTime.Time,
					Reason: signatureReason.Reason,
				})
			}
		}

		attachments, err := pdf.PdfiumInstance.GetAttachments(&requests.GetAttachments{
			Document: document.Document,
		})
		if err != nil {
			handleError(cmd, fmt.Errorf("could not get signature count for PDF %s: %w\n", args[0], newPdfiumError(err)), ExitCodePdfiumError)
			return
		}

		pdfInfo.Attachments = attachments.Attachments

		if outputType == "json" {
			outputJson, _ := json.MarshalIndent(pdfInfo, "", "  ")
			cmd.Println(string(outputJson))
		} else {
			yesNo := func(in bool) string {
				if in {
					return "Allowed"
				}
				return "Forbidden"
			}

			cmd.Printf("PDF Version: %s\n", pdfInfo.Version)

			// @todo: fix this when metadata has been properly fixed.
			if len(pdfInfo.Metadata) > 0 {
				cmd.Printf("Metadata:\n")
				for i := range pdfInfo.Metadata {
					cmd.Printf(" -  %s: %s\n", pdfInfo.Metadata[i].Tag, pdfInfo.Metadata[i].Value)
				}
			}

			cmd.Printf("Page count: %d\nPage size (in points (WxH), one point is 1/72 inch (around 0.3528 mm)):\n", pageCount.PageCount)

			for i := range pdfInfo.Pages {
				cmd.Printf(" - Page %d, size: %.2f x %.2f, label: %s\n", pdfInfo.Pages[i].Number, pdfInfo.Pages[i].Width, pdfInfo.Pages[i].Height, pdfInfo.Pages[i].Label)
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

			cmd.Printf("Security Handler Revision: %d", securityHandlerRevision.SecurityHandlerRevision)
			if securityHandlerRevision.SecurityHandlerRevision == -1 {
				cmd.Printf(" (no protection)")
			}
			cmd.Printf("\n")

			if len(pdfInfo.Signatures) > 0 {
				cmd.Printf("Signatures:\n")
				for i := range pdfInfo.Signatures {
					time := ""
					if pdfInfo.Signatures[i].Time != nil {
						time = *pdfInfo.Signatures[i].Time
					}

					reason := ""
					if pdfInfo.Signatures[i].Reason != nil {
						reason = *pdfInfo.Signatures[i].Reason
					}
					cmd.Printf(" - Signature %d, timestamp: %s, reason: %s\n", i+1, time, reason)
				}
			}

			if len(pdfInfo.Attachments) > 0 {
				cmd.Printf("Attachments:\n")
				for i := 0; i < len(pdfInfo.Attachments); i++ {
					valueTexts := []string{}
					for valueI := i; valueI < len(pdfInfo.Attachments[i].Values); valueI++ {
						valueTexts = append(valueTexts, fmt.Sprintf("%s (type %d): %s", pdfInfo.Attachments[i].Values[valueI].Key, pdfInfo.Attachments[i].Values[valueI].ValueType, pdfInfo.Attachments[i].Values[valueI].StringValue))
					}
					cmd.Printf(" - Attachment %d, name: %s, values: %s\n", i+1, pdfInfo.Attachments[i].Name, strings.Join(valueTexts, ", "))
				}
			}
		}
	},
}
