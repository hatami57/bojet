package bot

import "gopkg.in/telebot.v4"

type Page struct {
	ID       int
	Code     *string
	Title    string
	Items    []*PageItem
	IsPublic bool
}

type PageItem struct {
	Code              *string `json:"code"`
	Title             string  `json:"title"`
	ShowPageID        *int    `json:"showPageId"`
	ForwardMessageIDs []int64 `json:"forwardMessageIds"`

	ShowPage *Page
}

const PageBackText = "🔙 Back"

func (p *Page) GetKeyboard(withBack bool) *telebot.ReplyMarkup {
	keyboard := &telebot.ReplyMarkup{ResizeKeyboard: true, OneTimeKeyboard: true}
	var rows []telebot.Row

	if withBack {
		rows = append(rows, keyboard.Row(keyboard.Text(PageBackText)))
	}

	for _, item := range p.Items {
		rows = append(rows, keyboard.Row(keyboard.Text(item.Title)))
	}

	keyboard.Reply(rows...)

	return keyboard
}
