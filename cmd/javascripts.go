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
	addGenericPDFOptions(javascriptsCmd)
	rootCmd.AddCommand(javascriptsCmd)
}

var javascriptsCmd = &cobra.Command{
	Use:   "javascripts [input] [output-folder]",
	Short: "Extract the javascripts of a PDF",
	Long:  "Extract the javascripts of a PDF and store them as file.\n[input] can either be a file path or - for stdin.",
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

		javascripts, err := pdf.PdfiumInstance.GetJavaScriptActions(&requests.GetJavaScriptActions{
			Document: document.Document,
		})
		if err != nil {
			cmd.PrintErrf("could not get javascript count for PDF %s: %w\n", args[0], err)
			return
		}

		if len(javascripts.JavaScriptActions) > 0 {
			for i := 0; i < len(javascripts.JavaScriptActions); i++ {
				filePath := path.Join(args[1], fmt.Sprintf("%s.js", javascripts.JavaScriptActions[i].Name))
				outFile, err := os.Create(filePath)
				if err != nil {
					cmd.PrintErrf("could not create output file for javascript %d for PDF %s: %w\n", i, args[0], err)
					return
				}

				outFile.Write([]byte(javascripts.JavaScriptActions[i].Script))
				outFile.Close()

				cmd.Printf("Exported javascript %d into %s\n", i+1, filePath)
			}
		}
	},
}
