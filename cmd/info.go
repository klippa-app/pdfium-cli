package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/klippa-app/pdfium-cli/pdf"

	"github.com/klippa-app/go-pdfium/enums"
	"github.com/klippa-app/go-pdfium/references"
	"github.com/klippa-app/go-pdfium/requests"
	"github.com/klippa-app/go-pdfium/responses"
	"github.com/spf13/cobra"
)

var withObjects bool

func init() {
	addGenericPDFOptions(infoCmd)
	infoCmd.Flags().StringVarP(&outputType, "output-type", "", "text", "The file type to output, text or json")
	infoCmd.Flags().BoolVarP(&withObjects, "with-objects", "", false, "Count page objects by type (path, text, image, shading, form). Descends into form XObjects recursively.")
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

		type pdfObjectCounts struct {
			Paths    int
			Texts    int
			Images   int
			Shadings int
			Forms    int
			Unknown  int
		}

		type pdfPageObjects struct {
			Counts           pdfObjectCounts
			HasVectorContent bool
			HasRasterContent bool
		}

		type pdfPage struct {
			Number   int
			Width    float64
			Height   float64
			Label    string
			Rotation int
			Objects  *pdfPageObjects
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
			Objects                 *pdfPageObjects
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

		// walkObject counts a single page object and recurses into form XObjects.
		var walkObject func(*pdfObjectCounts, references.FPDF_PAGEOBJECT) error
		walkObject = func(counts *pdfObjectCounts, obj references.FPDF_PAGEOBJECT) error {
			objType, err := pdf.PdfiumInstance.FPDFPageObj_GetType(&requests.FPDFPageObj_GetType{
				PageObject: obj,
			})
			if err != nil {
				return err
			}
			switch objType.Type {
			case enums.FPDF_PAGEOBJ_PATH:
				counts.Paths++
			case enums.FPDF_PAGEOBJ_TEXT:
				counts.Texts++
			case enums.FPDF_PAGEOBJ_IMAGE:
				counts.Images++
			case enums.FPDF_PAGEOBJ_SHADING:
				counts.Shadings++
			case enums.FPDF_PAGEOBJ_FORM:
				counts.Forms++
				formObjCount, err := pdf.PdfiumInstance.FPDFFormObj_CountObjects(&requests.FPDFFormObj_CountObjects{
					PageObject: obj,
				})
				if err != nil {
					return err
				}
				for j := 0; j < formObjCount.Count; j++ {
					childObj, err := pdf.PdfiumInstance.FPDFFormObj_GetObject(&requests.FPDFFormObj_GetObject{
						PageObject: obj,
						Index:      uint64(j),
					})
					if err != nil {
						return err
					}
					if err := walkObject(counts, childObj.PageObject); err != nil {
						return err
					}
				}
			default:
				counts.Unknown++
			}
			return nil
		}

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

			rotation, err := pdf.PdfiumInstance.FPDFPage_GetRotation(&requests.FPDFPage_GetRotation{
				Page: requests.Page{
					ByIndex: &requests.PageByIndex{
						Document: document.Document,
						Index:    i,
					},
				},
			})
			if err != nil {
				handleError(cmd, fmt.Errorf("could not get page rotation for page %d of PDF %s: %w\n", i+1, args[0], newPdfiumError(err)), ExitCodePdfiumError)
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

			newPage := pdfPage{
				Number:   i + 1,
				Width:    pageSize.Width,
				Height:   pageSize.Height,
				Label:    label,
				Rotation: int(rotation.PageRotation) * 90,
			}

			if withObjects {
				loadedPage, err := pdf.PdfiumInstance.FPDF_LoadPage(&requests.FPDF_LoadPage{
					Document: document.Document,
					Index:    i,
				})
				if err != nil {
					handleError(cmd, fmt.Errorf("could not load page %d for PDF %s: %w\n", i+1, args[0], newPdfiumError(err)), ExitCodePdfiumError)
					return
				}

				counts := &pdfObjectCounts{}
				objCount, err := pdf.PdfiumInstance.FPDFPage_CountObjects(&requests.FPDFPage_CountObjects{
					Page: requests.Page{ByReference: &loadedPage.Page},
				})
				if err != nil {
					pdf.PdfiumInstance.FPDF_ClosePage(&requests.FPDF_ClosePage{Page: loadedPage.Page})
					handleError(cmd, fmt.Errorf("could not count objects for page %d of PDF %s: %w\n", i+1, args[0], newPdfiumError(err)), ExitCodePdfiumError)
					return
				}

				var walkErr error
				for j := 0; j < objCount.Count && walkErr == nil; j++ {
					obj, err := pdf.PdfiumInstance.FPDFPage_GetObject(&requests.FPDFPage_GetObject{
						Page:  requests.Page{ByReference: &loadedPage.Page},
						Index: j,
					})
					if err != nil {
						walkErr = err
						break
					}
					walkErr = walkObject(counts, obj.PageObject)
				}

				pdf.PdfiumInstance.FPDF_ClosePage(&requests.FPDF_ClosePage{Page: loadedPage.Page})

				if walkErr != nil {
					handleError(cmd, fmt.Errorf("could not walk objects for page %d of PDF %s: %w\n", i+1, args[0], newPdfiumError(walkErr)), ExitCodePdfiumError)
					return
				}

				newPage.Objects = &pdfPageObjects{
					Counts:           *counts,
					HasVectorContent: counts.Paths > 0 || counts.Shadings > 0,
					HasRasterContent: counts.Images > 0,
				}
			}

			pdfInfo.Pages = append(pdfInfo.Pages, newPage)
		}

		if withObjects {
			total := &pdfObjectCounts{}
			for _, p := range pdfInfo.Pages {
				if p.Objects != nil {
					total.Paths += p.Objects.Counts.Paths
					total.Texts += p.Objects.Counts.Texts
					total.Images += p.Objects.Counts.Images
					total.Shadings += p.Objects.Counts.Shadings
					total.Forms += p.Objects.Counts.Forms
					total.Unknown += p.Objects.Counts.Unknown
				}
			}
			pdfInfo.Objects = &pdfPageObjects{
				Counts:           *total,
				HasVectorContent: total.Paths > 0 || total.Texts > 0 || total.Shadings > 0,
				HasRasterContent: total.Images > 0,
			}
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
				if pdfInfo.Pages[i].Objects != nil {
					o := pdfInfo.Pages[i].Objects
					cmd.Printf("   Objects: paths=%d, text=%d, images=%d, shading=%d, forms=%d | vector=%v, raster=%v\n",
						o.Counts.Paths, o.Counts.Texts, o.Counts.Images, o.Counts.Shadings, o.Counts.Forms,
						o.HasVectorContent, o.HasRasterContent)
				}
			}

			if pdfInfo.Objects != nil {
				o := pdfInfo.Objects
				cmd.Printf("Total objects: paths=%d, text=%d, images=%d, shading=%d, forms=%d | vector=%v, raster=%v\n",
					o.Counts.Paths, o.Counts.Texts, o.Counts.Images, o.Counts.Shadings, o.Counts.Forms,
					o.HasVectorContent, o.HasRasterContent)
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
