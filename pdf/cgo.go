//go:build usecgo
// +build usecgo

package pdf

import (
	"time"

	"github.com/klippa-app/go-pdfium/single_threaded"
)

func LoadPdfium() error {
	if isLoaded {
		return nil
	}

	var err error

	// Init the PDFium library and return the instance to open documents.
	pool = single_threaded.Init(single_threaded.Config{})

	PdfiumInstance, err = pool.GetInstance(time.Second * 30)
	if err != nil {
		return err
	}

	isLoaded = true

	return nil
}
