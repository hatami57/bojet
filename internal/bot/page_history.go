package bot

type PageHistory []*Page

func (h *PageHistory) Push(page *Page) {
	*h = append(*h, page)
}

func (h *PageHistory) Pop() *Page {
	if h.IsEmpty() {
		return nil
	}

	n := len(*h)
	page := (*h)[n-1]
	*h = (*h)[:n-1]

	return page
}

func (h *PageHistory) IsEmpty() bool {
	return h == nil || len(*h) == 0
}
