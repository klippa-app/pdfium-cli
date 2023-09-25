package pdf

import (
	"testing"
)

func TestNormalizePageRange(t *testing.T) {
	tests := []struct {
		name      string
		pageCount int
		pageRange string
		want      string
		wantErr   string
	}{
		{
			"test first-last",
			5,
			"first-last",
			"1,2,3,4,5",
			"",
		},
		{
			"test page-range",
			5,
			"1-5",
			"1,2,3,4,5",
			"",
		},
		{
			"test out of range page-range",
			5,
			"1-10",
			"1,2,3,4,5",
			"10 is not a valid page number, the document has 5 page(s)",
		},
		{
			"test out of range page-range",
			20,
			"1,2,4-22",
			"1,2,3,4,5",
			"22 is not a valid page number, the document has 20 page(s)",
		},
		{
			"test reverse page-range",
			5,
			"1-r2",
			"1,2,3",
			"",
		},
		{
			"test negative reverse page-range",
			5,
			"1-r6",
			"1,2,3",
			"-1 is not a valid page number, the document has 5 page(s)",
		},
		{
			"test removal of duplicate pages",
			5,
			"1-3,first-last,2,3",
			"1,2,3,4,5",
			"",
		},
	}

	for i := range tests {
		t.Run(tests[i].name, func(t *testing.T) {
			normalizedPageRange, _, err := NormalizePageRange(tests[i].pageCount, tests[i].pageRange)
			if tests[i].wantErr == "" && err != nil {
				t.Errorf("expected no error but got error %s", err.Error())
			} else if tests[i].wantErr != "" && err == nil {
				t.Errorf("expected error %s but got no error", tests[i].wantErr)
			} else if tests[i].wantErr != "" && err != nil && err.Error() != tests[i].wantErr {
				t.Errorf("expected error %s but got error %s", tests[i].wantErr, err.Error())
			} else if err == nil && tests[i].want != *normalizedPageRange {
				t.Errorf("expected %s but got %s", tests[i].want, *normalizedPageRange)
			}
		})
	}
}
