package client

import "testing"

func TestVndbIDOfPicksTheWorkAnchor(t *testing.T) {
	cases := []struct {
		name string
		refs []catalogRef
		want string
	}{
		{
			name: "release anchor first — the work anchor still wins",
			refs: []catalogRef{
				{Source: "vndb", ExternalID: "r123"},
				{Source: "vndb", ExternalID: "v456"},
			},
			want: "v456",
		},
		{
			name: "release anchors only — no work id to report",
			refs: []catalogRef{
				{Source: "vndb", ExternalID: "r123"},
				{Source: "vndb", ExternalID: "r124"},
			},
			want: "",
		},
		{
			name: "work anchor alone",
			refs: []catalogRef{{Source: "vndb", ExternalID: "v42"}},
			want: "v42",
		},
		{
			name: "other sources are skipped, whatever they are keyed on",
			refs: []catalogRef{
				{Source: "dlsite", ExternalID: "RJ01234"},
				{Source: "bangumi", ExternalID: "v42"},
				{Source: "vndb", ExternalID: "v42"},
			},
			want: "v42",
		},
		{
			name: "malformed vndb ids are not work anchors",
			refs: []catalogRef{
				{Source: "vndb", ExternalID: "v"},
				{Source: "vndb", ExternalID: "v12a"},
				{Source: "vndb", ExternalID: "vn3"},
				{Source: "vndb", ExternalID: ""},
			},
			want: "",
		},
		{name: "no refs at all", refs: nil, want: ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := vndbIDOf(tc.refs); got != tc.want {
				t.Errorf("vndbIDOf = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestNumericRefOfReadsTheFirstNumericAnchor(t *testing.T) {
	work1781 := []catalogRef{
		{Source: "vndb", ExternalID: "v8437"},
		{Source: "bangumi", ExternalID: "26450"},
		{Source: "erogamescape", ExternalID: "16280"},
	}
	cases := []struct {
		name   string
		refs   []catalogRef
		source string
		want   int
	}{
		{name: "bangumi subject", refs: work1781, source: "bangumi", want: 26450},
		{name: "erogamescape game", refs: work1781, source: "erogamescape", want: 16280},
		{
			name: "a later row of the same source does not displace the first",
			refs: []catalogRef{
				{Source: "erogamescape", ExternalID: "16280"},
				{Source: "erogamescape", ExternalID: "12345"},
			},
			source: "erogamescape",
			want:   16280,
		},
		{
			name: "non-numeric ids are skipped",
			refs: []catalogRef{
				{Source: "bangumi", ExternalID: ""},
				{Source: "bangumi", ExternalID: "s26450"},
				{Source: "bangumi", ExternalID: "+26450"},
				{Source: "bangumi", ExternalID: "-1"},
				{Source: "bangumi", ExternalID: "0"},
				{Source: "bangumi", ExternalID: "99999999999999999999"},
				{Source: "bangumi", ExternalID: "26450"},
			},
			source: "bangumi",
			want:   26450,
		},
		{
			name: "another source's id is never borrowed",
			refs: []catalogRef{
				{Source: "vndb", ExternalID: "v8437"},
				{Source: "erogamescape", ExternalID: "16280"},
			},
			source: "bangumi",
		},
		{name: "no refs at all", source: "erogamescape"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := numericRefOf(tc.refs, tc.source)
			switch {
			case tc.want == 0 && got != nil:
				t.Errorf("numericRefOf = %d, want nil", *got)
			case tc.want != 0 && (got == nil || *got != tc.want):
				t.Errorf("numericRefOf = %v, want %d", got, tc.want)
			}
		})
	}
}
