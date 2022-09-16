package pdf

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/klippa-app/go-pdfium"
	"github.com/klippa-app/go-pdfium/single_threaded"
)

// Be sure to close pools/instances when you're done with them.
var pool pdfium.Pool
var PdfiumInstance pdfium.Pdfium
var isLoaded bool

func LoadPdfium() error {
	if isLoaded {
		return nil
	}

	// Init the PDFium library and return the instance to open documents.
	pool = single_threaded.Init(single_threaded.Config{})

	var err error
	PdfiumInstance, err = pool.GetInstance(time.Second * 30)
	if err != nil {
		return err
	}

	isLoaded = true

	return nil
}

func ClosePdfium() {
	if !isLoaded {
		return
	}

	PdfiumInstance.Close()
	isLoaded = false
}

// NormalizePageRange converts a page range into separate page numbers so that
// we can support a more range of page range options compared to Pdfium. Pdfium only
// supports simple instructions like 1-5 or just a page number. This method
// can automatically calculate ends and reverse pages for example.
// This way we can also properly validate page ranges.
func NormalizePageRange(pageCount int, pageRange string, allowDuplicates bool) (*string, *int, error) {
	calculatedPageCount := 0
	var calculatedPageNumbers []string
	seenPageNumbers := map[int]bool{}

	pageRanges := strings.Split(pageRange, ",")
	for i := range pageRanges {
		pageRangeParts := strings.Split(pageRanges[i], "-")
		if len(pageRangeParts) == 0 || len(pageRangeParts) > 2 {
			return nil, nil, errors.New("a page range must contain 1 or 2 components")
		}

		var pageNumbers []int
		for pageRangePartI := range pageRangeParts {
			if pageRangeParts[pageRangePartI] == "first" {
				if !allowDuplicates {
					if _, ok := seenPageNumbers[1]; ok {
						continue
					}
				}

				seenPageNumbers[1] = true
				pageNumbers = append(pageNumbers, 1)
			} else if pageRangeParts[pageRangePartI] == "last" {
				if !allowDuplicates {
					if _, ok := seenPageNumbers[pageCount]; ok {
						continue
					}
				}

				seenPageNumbers[pageCount] = true
				pageNumbers = append(pageNumbers, pageCount)
			} else if strings.HasPrefix(pageRangeParts[pageRangePartI], "r") {
				parsedPageNumber, err := strconv.Atoi(strings.TrimPrefix(pageRangeParts[pageRangePartI], "r"))
				if err != nil {
					return nil, nil, fmt.Errorf("%s is not a valid page number", strings.TrimPrefix(pageRangeParts[pageRangePartI], "r"))
				}

				if pageCount-parsedPageNumber < 1 || pageCount-parsedPageNumber > pageCount {
					return nil, nil, fmt.Errorf("%s is not a valid page number, the document has %d page(s)", strings.TrimPrefix(pageRangeParts[pageRangePartI], "r"), pageCount)
				}

				if !allowDuplicates {
					if _, ok := seenPageNumbers[pageCount-parsedPageNumber]; ok {
						continue
					}
				}

				seenPageNumbers[pageCount-parsedPageNumber] = true
				pageNumbers = append(pageNumbers, pageCount-parsedPageNumber)
			} else {
				parsedPageNumber, err := strconv.Atoi(pageRangeParts[pageRangePartI])
				if err != nil {
					return nil, nil, fmt.Errorf("%s is not a valid page number", pageRangeParts[pageRangePartI])
				}

				if parsedPageNumber < 1 || parsedPageNumber > pageCount {
					return nil, nil, fmt.Errorf("%s is not a valid page number, the document has %d page(s)", pageRangeParts[pageRangePartI], pageCount)
				}

				if !allowDuplicates {
					if _, ok := seenPageNumbers[parsedPageNumber]; ok {
						continue
					}
				}

				seenPageNumbers[parsedPageNumber] = true
				pageNumbers = append(pageNumbers, parsedPageNumber)
			}
		}

		if len(pageNumbers) == 0 {
			continue
		} else if len(pageNumbers) == 1 {
			// Only 1 page number.
			calculatedPageNumbers = append(calculatedPageNumbers, strconv.Itoa(pageNumbers[0]))
		} else {
			// A page range, a start and end number. Tokens should be replaced by earlier logic.
			for i := pageNumbers[0]; i <= pageNumbers[1]; i++ {
				calculatedPageNumbers = append(calculatedPageNumbers, strconv.Itoa(i))
			}
		}
	}

	pageRange = strings.Join(calculatedPageNumbers, ",")
	calculatedPageCount = len(calculatedPageNumbers)

	return &pageRange, &calculatedPageCount, nil
}
