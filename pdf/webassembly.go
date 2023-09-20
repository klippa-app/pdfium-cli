//go:build !pdfium_cli_use_cgo

package pdf

import (
	"time"

	"github.com/klippa-app/go-pdfium/webassembly"
)

func LoadPdfium() error {
	if isLoaded {
		return nil
	}

	var err error
	// Init the PDFium library and return the instance to open documents.
	pool, err = webassembly.Init(webassembly.Config{
		MinIdle:  1,
		MaxIdle:  1,
		MaxTotal: 1,
	})
	if err != nil {
		return err
	}

	PdfiumInstance, err = pool.GetInstance(time.Second * 30)
	if err != nil {
		return err
	}

	isLoaded = true

	return nil
}
