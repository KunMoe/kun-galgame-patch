package catalogv2

type List[T any] struct {
	Object     string                   `json:"object"`
	Items      []T                      `json:"items"`
	NextCursor *string                  `json:"next_cursor"`
	Total      *int64                   `json:"total"`
	Missing    *[]string                `json:"missing"`
	Facets     *map[string][]FacetValue `json:"facets"`
}

func (l List[T]) Next() string {
	if l.NextCursor == nil {
		return ""
	}
	return *l.NextCursor
}

func (l List[T]) Count() int64 {
	if l.Total != nil {
		return *l.Total
	}
	return int64(len(l.Items))
}

type FacetValue struct {
	Value       string `json:"value"`
	DisplayName string `json:"display_name"`
	Count       int    `json:"count"`
}

type LocalizedText struct {
	Value     string `json:"value"`
	IsMachine bool   `json:"is_machine"`
}

type Image struct {
	URL       string  `json:"url"`
	Hash      string  `json:"hash"`
	Width     *int    `json:"width"`
	Height    *int    `json:"height"`
	Thumbhash *string `json:"thumbhash"`
	Sexual    *string `json:"sexual"`
	Violence  *string `json:"violence"`
	Source    string  `json:"source"`
}

type EntityName struct {
	Lang      string `json:"lang"`
	Value     string `json:"value"`
	AliasKind string `json:"alias_kind"`
	IsMachine bool   `json:"is_machine"`
}

type Link struct {
	Source     string `json:"source"`
	URL        string `json:"url"`
	IsIdentity bool   `json:"is_identity"`
}

type Ref struct {
	Source     string `json:"source"`
	ExternalID string `json:"external_id"`
}

type Claim struct {
	Site         string `json:"site"`
	SiteWorkID   string `json:"site_work_id"`
	State        string `json:"state"`
	ContentLimit string `json:"content_limit"`
}
