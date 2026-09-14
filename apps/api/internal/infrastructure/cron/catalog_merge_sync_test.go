package cron

import "testing"

func TestActionForCoversEveryPageShape(t *testing.T) {
	cases := []struct {
		name             string
		oldPage, newPage bool
		want             redirectAction
	}{
		{
			name:    "both numbers are pages, so the retired one folds in",
			oldPage: true, newPage: true, want: redirectFold,
		},
		{
			name:    "only the retired number is a page, so the page follows its work",
			oldPage: true, newPage: false, want: redirectRenumber,
		},
		{
			name:    "only the survivor is a page, so /galgame/<old> gains a 301",
			oldPage: false, newPage: true, want: redirectLedger,
		},
		{
			name:    "a work this site never carried leaves no ledger row behind",
			oldPage: false, newPage: false, want: redirectSkip,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := actionFor(tc.oldPage, tc.newPage); got != tc.want {
				t.Errorf("actionFor(%v, %v) = %v, want %v", tc.oldPage, tc.newPage, got, tc.want)
			}
		})
	}
}
