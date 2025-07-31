package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/klippa-app/pdfium-cli/pdf"

	"github.com/klippa-app/go-pdfium/enums"
	"github.com/klippa-app/go-pdfium/requests"
	"github.com/spf13/cobra"
)

func init() {
	addGenericPDFOptions(formCmd)
	formCmd.Flags().StringVarP(&outputType, "output-type", "", "text", "The file type to output, text or json")
	rootCmd.AddCommand(formCmd)
}

var formCmd = &cobra.Command{
	Use:   "form [input] [output]",
	Short: "Get the form of a PDF",
	Long:  "Get the form of a PDF and its pages, like form fields, the values and options.\n[input] can either be a file path or - for stdin.\n[output] can either be a file path or - for stdout (default).",
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

		type pdfFormFieldFlagsStruct struct {
			ReadOnly bool
			Required bool
			NoExport bool
		}

		type pdfFormFieldStruct struct {
			PageNumber int
			Type       string
			Name       string
			Value      *string
			Values     []string
			IsChecked  *bool
			ToolTip    string
			Options    []string
			Flags      pdfFormFieldFlagsStruct
		}

		type pdfFormStruct struct {
			Fields []pdfFormFieldStruct
		}

		pdfForm := &pdfFormStruct{
			Fields: []pdfFormFieldStruct{},
		}

		pageCount, err := pdf.PdfiumInstance.FPDF_GetPageCount(&requests.FPDF_GetPageCount{
			Document: document.Document,
		})
		if err != nil {
			handleError(cmd, fmt.Errorf("could not get page count for PDF %s: %w\n", args[0], newPdfiumError(err)), ExitCodePdfiumError)
			return
		}

		for i := 0; i < pageCount.PageCount; i++ {
			pageForm, err := pdf.PdfiumInstance.GetForm(&requests.GetForm{
				Page: requests.Page{
					ByIndex: &requests.PageByIndex{
						Document: document.Document,
						Index:    i,
					},
				},
			})
			if err != nil {
				if isExperimentalError(err) {
					handleError(cmd, fmt.Errorf("Form support is not enabled in your build, build with the build tag pdfium_experimental to enable!\n"), ExitCodeExperimental)
					return
				}
				handleError(cmd, fmt.Errorf("could not get page form for page %d of PDF %s: %w\n", i+1, args[0], newPdfiumError(err)), ExitCodePdfiumError)
				return
			}

			for fieldI := range pageForm.Fields {
				pdfForm.Fields = append(pdfForm.Fields, pdfFormFieldStruct{
					PageNumber: i + 1,
					Type:       fieldTypeToString(pageForm.Fields[fieldI].Type),
					Name:       pageForm.Fields[fieldI].Name,
					Value:      pageForm.Fields[fieldI].Value,
					Values:     pageForm.Fields[fieldI].Values,
					IsChecked:  pageForm.Fields[fieldI].IsChecked,
					ToolTip:    pageForm.Fields[fieldI].ToolTip,
					Options:    pageForm.Fields[fieldI].Options,
					Flags: pdfFormFieldFlagsStruct{
						ReadOnly: pageForm.Fields[fieldI].Flags.ReadOnly,
						Required: pageForm.Fields[fieldI].Flags.Required,
						NoExport: pageForm.Fields[fieldI].Flags.NoExport,
					},
				})
			}
		}

		if outputType == "json" {
			outputJson, _ := json.MarshalIndent(pdfForm, "", "  ")
			cmd.Println(string(outputJson))
		} else {
			yesNo := func(in bool) string {
				if in {
					return "Yes"
				}
				return "No"
			}

			if len(pdfForm.Fields) > 0 {
				cmd.Printf("Form fields:\n")
				for i := range pdfForm.Fields {
					cmd.Printf("- Name: %s\n", pdfForm.Fields[i].Name)
					if pdfForm.Fields[i].ToolTip != "" {
						cmd.Printf("  ToolTip: %s\n", pdfForm.Fields[i].ToolTip)
					}
					cmd.Printf("  Page number: %d\n", pdfForm.Fields[i].PageNumber)
					cmd.Printf("  Field type: %s\n", pdfForm.Fields[i].Type)
					if pdfForm.Fields[i].IsChecked != nil {
						cmd.Printf("  Checked: %s\n", yesNo(*pdfForm.Fields[i].IsChecked))
					}

					if pdfForm.Fields[i].Value != nil {
						cmd.Printf("  Value: %s\n", *pdfForm.Fields[i].Value)
					}
					if pdfForm.Fields[i].Values != nil {
						if len(pdfForm.Fields[i].Values) == 0 {
							cmd.Printf("  Values: None\n")
						} else {
							cmd.Printf("  Values:\n   - %s\n", strings.Join(pdfForm.Fields[i].Values, "\n   - "))
						}
					}
					if pdfForm.Fields[i].Options != nil {
						if len(pdfForm.Fields[i].Options) == 0 {
							cmd.Printf("  Options: None\n")
						} else {
							cmd.Printf("  Options:\n   - %s\n", strings.Join(pdfForm.Fields[i].Options, "\n   - "))
						}
					}

					cmd.Printf("  Flags:\n")
					cmd.Printf("   - Read Only: %s\n", yesNo(pdfForm.Fields[i].Flags.ReadOnly))
					cmd.Printf("   - Required: %s\n", yesNo(pdfForm.Fields[i].Flags.Required))
					cmd.Printf("   - No Export: %s\n", yesNo(pdfForm.Fields[i].Flags.NoExport))
					cmd.Printf("\n")
				}
			}
		}
	},
}

func fieldTypeToString(fieldType enums.FPDF_FORMFIELD_TYPE) string {
	switch fieldType {
	case enums.FPDF_FORMFIELD_TYPE_PUSHBUTTON:
		return "PUSHBUTTON"
	case enums.FPDF_FORMFIELD_TYPE_CHECKBOX:
		return "CHECKBOX"
	case enums.FPDF_FORMFIELD_TYPE_RADIOBUTTON:
		return "RADIOBUTTON"
	case enums.FPDF_FORMFIELD_TYPE_COMBOBOX:
		return "COMBOBOX"
	case enums.FPDF_FORMFIELD_TYPE_LISTBOX:
		return "LISTBOX"
	case enums.FPDF_FORMFIELD_TYPE_TEXTFIELD:
		return "TEXTFIELD"
	case enums.FPDF_FORMFIELD_TYPE_SIGNATURE:
		return "SIGNATURE"
	case enums.FPDF_FORMFIELD_TYPE_XFA:
		return "XFA"
	case enums.FPDF_FORMFIELD_TYPE_XFA_CHECKBOX:
		return "XFA_CHECKBOX"
	case enums.FPDF_FORMFIELD_TYPE_XFA_COMBOBOX:
		return "XFA_COMBOBOX"
	case enums.FPDF_FORMFIELD_TYPE_XFA_IMAGEFIELD:
		return "XFA_IMAGEFIELD"
	case enums.FPDF_FORMFIELD_TYPE_XFA_LISTBOX:
		return "XFA_LISTBOX"
	case enums.FPDF_FORMFIELD_TYPE_XFA_PUSHBUTTON:
		return "XFA_PUSHBUTTON"
	case enums.FPDF_FORMFIELD_TYPE_XFA_SIGNATURE:
		return "XFA_SIGNATURE"
	case enums.FPDF_FORMFIELD_TYPE_XFA_TEXTFIELD:
		return "XFA_TEXTFIELD"
	default:
		return "UNKNOWN"
	}
}
