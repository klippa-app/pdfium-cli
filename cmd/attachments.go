package cmd

import (
	"fmt"
	"os"
	"path"

	"github.com/klippa-app/pdfium-cli/pdf"

	"github.com/klippa-app/go-pdfium/requests"
	"github.com/spf13/cobra"
)

func init() {
	addGenericPDFOptions(attachmentsCmd)
	rootCmd.AddCommand(attachmentsCmd)
}

var attachmentsCmd = &cobra.Command{
	Use:   "attachments [input] [output-folder]",
	Short: "Extract the attachments of a PDF",
	Long:  "Extract the attachments of a PDF and store them as file.\n[input] can either be a file path or - for stdin.",
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

		attachments, err := pdf.PdfiumInstance.GetAttachments(&requests.GetAttachments{
			Document: document.Document,
		})
		if err != nil {
			cmd.PrintErrf("could not get attachment count for PDF %s: %w\n", args[0], err)
			return
		}

		if len(attachments.Attachments) > 0 {
			for i := 0; i < len(attachments.Attachments); i++ {
				filePath := path.Join(args[1], attachments.Attachments[i].Name)
				outFile, err := os.Create(filePath)
				if err != nil {
					cmd.PrintErrf("could not create output file for attachment %d for PDF %s: %w\n", i, args[0], err)
					return
				}

				outFile.Write(attachments.Attachments[i].Content)
				outFile.Close()

				cmd.Printf("Exported attachment %d into %s\n", i+1, filePath)
			}
		}
	},
}
