package pdf

import (
	"errors"
	"fmt"
	"github.com/klippa-app/go-pdfium"
	"strconv"
	"strings"
)

// Be sure to close pools/instances when you're done with them.
var pool pdfium.Pool
var PdfiumInstance pdfium.Pdfium
var isLoaded bool

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
func NormalizePageRange(pageCount int, pageRange string, ignoreInvalidPages bool) (*string, *int, error) {
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
				pageNumbers = append(pageNumbers, 1)
			} else if pageRangeParts[pageRangePartI] == "last" {
				pageNumbers = append(pageNumbers, pageCount)
			} else if strings.HasPrefix(pageRangeParts[pageRangePartI], "r") {
				parsedPageNumber, err := strconv.Atoi(strings.TrimPrefix(pageRangeParts[pageRangePartI], "r"))
				if err != nil {
					return nil, nil, fmt.Errorf("%s is not a valid page number", strings.TrimPrefix(pageRangeParts[pageRangePartI], "r"))
				}

				pageNumbers = append(pageNumbers, pageCount-parsedPageNumber)
			} else {
				parsedPageNumber, err := strconv.Atoi(pageRangeParts[pageRangePartI])
				if err != nil {
					return nil, nil, fmt.Errorf("%s is not a valid page number", pageRangeParts[pageRangePartI])
				}

				pageNumbers = append(pageNumbers, parsedPageNumber)
			}
		}
		if len(pageNumbers) == 0 {
			continue
		} else if len(pageNumbers) == 1 {
			if pageNumbers[0] < 1 || pageNumbers[0] > pageCount {
				if ignoreInvalidPages {
					continue
				}
				return nil, nil, fmt.Errorf("%d is not a valid page number, the document has %d page(s)", pageNumbers[0], pageCount)
			}

			_, seen := seenPageNumbers[pageNumbers[0]]
			if !seen {
				// Only 1 page number.
				seenPageNumbers[pageNumbers[0]] = true
				calculatedPageNumbers = append(calculatedPageNumbers, strconv.Itoa(pageNumbers[0]))
			}
		} else {
			// If the end page number is lower than the start page number,
			// ignore the whole page range.
			if pageNumbers[1] < pageNumbers[0] {
				if ignoreInvalidPages {
					continue
				}
				return nil, nil, fmt.Errorf("%d is not a valid page number, the document has %d page(s)", pageNumbers[1], pageCount)
			}

			// A page range, a start and end number. Tokens should be replaced by earlier logic.
			for i := pageNumbers[0]; i <= pageNumbers[1]; i++ {
				if i < 1 || i > pageCount {
					if ignoreInvalidPages {
						continue
					}
					return nil, nil, fmt.Errorf("%d is not a valid page number, the document has %d page(s)", i, pageCount)
				}

				_, seen := seenPageNumbers[i]
				if !seen {
					seenPageNumbers[i] = true
					calculatedPageNumbers = append(calculatedPageNumbers, strconv.Itoa(i))
				}
			}
		}
	}

	if len(calculatedPageNumbers) == 0 {
		return nil, nil, fmt.Errorf("the page range(s) resulted in no valid pages")
	}

	pageRange = strings.Join(calculatedPageNumbers, ",")
	calculatedPageCount = len(calculatedPageNumbers)

	return &pageRange, &calculatedPageCount, nil
}
