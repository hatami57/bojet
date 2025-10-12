package bot

type Page struct {
	ID       int
	Title    string
	Items    []*PageItem
	IsPublic bool
}

type PageItem struct {
	Title             string  `json:"title"`
	ShowPageID        *int    `json:"showPageId"`
	ForwardMessageIDs []int64 `json:"forwardMessageIds"`
	IsPublic          bool    `json:"isPublic"`

	ShowPage *Page
}
