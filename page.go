package bojet

import "gopkg.in/telebot.v4"

// PageBackText is the button label used for back navigation.
const PageBackText = "🔙 Back"

// Page is a named menu screen with a list of items.
type Page struct {
	Title string
	items []*PageItem
}

// PageItem is a single entry in a Page.
type PageItem struct {
	title  string
	action pageAction
}

type pageAction interface {
	execute(c Context, b *Bot) error
}

// navAction navigates to a sub-page and pushes the current page onto history.
type navAction struct{ page *Page }

func (a *navAction) execute(c Context, b *Bot) error {
	u := c.BotUser()
	u.Session.PageHistory.Push(u.Session.CurrentPage)
	u.Session.CurrentPage = a.page
	return c.Send(a.page.Title, b.userKeyboard(u))
}

// handlerAction delegates to a user-supplied HandlerFunc.
type handlerAction struct{ fn HandlerFunc }

func (a *handlerAction) execute(c Context, b *Bot) error {
	return a.fn(c)
}

// NewPage creates a page with the given title and items.
//
//	statsPage := bojet.NewPage("📊 Statistics",
//	    bojet.ActionItem("Today", todayHandler),
//	    bojet.NavItem("Archive", archivePage),
//	)
func NewPage(title string, items ...*PageItem) *Page {
	return &Page{Title: title, items: items}
}

// NavItem creates a page item that navigates to the given sub-page.
func NavItem(title string, page *Page) *PageItem {
	return &PageItem{title: title, action: &navAction{page: page}}
}

// ActionItem creates a page item that runs a custom handler when pressed.
func ActionItem(title string, fn HandlerFunc) *PageItem {
	return &PageItem{title: title, action: &handlerAction{fn: fn}}
}

// processText matches the text against page items and executes the matching
// action. Returns (true, err) if matched, (false, nil) if not.
func (p *Page) processText(text string, c Context, b *Bot) (bool, error) {
	for _, item := range p.items {
		if item.title == text {
			return true, item.action.execute(c, b)
		}
	}
	return false, nil
}

// keyboard builds the reply keyboard for this page.
func (b *Bot) userKeyboard(u *User) *telebot.ReplyMarkup {
	if u == nil || u.Session == nil {
		return nil
	}

	rm := &telebot.ReplyMarkup{ResizeKeyboard: true, OneTimeKeyboard: true}
	var rows []telebot.Row

	if !u.Session.PageHistory.IsEmpty() {
		rows = append(rows, rm.Row(rm.Text(PageBackText)))
	}

	if u.Session.CurrentPage != nil {
		for _, item := range u.Session.CurrentPage.items {
			rows = append(rows, rm.Row(rm.Text(item.title)))
		}
	}

	if b.config.ContactAdmin {
		rows = append(rows, rm.Row(rm.Text(b.messages.ContactAdminButton)))
	}

	rm.Reply(rows...)
	return rm
}
