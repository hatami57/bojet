package bot

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"gopkg.in/telebot.v4"
)

func (b *SonateBot) loadUser(id int64) (*User, error) {
	user := NewUser(b.userHomePage, b.UserCacheExpirationDuration)

	query := "SELECT tg_id, first_name, last_name, username, phone_number, is_confirmed FROM users WHERE tg_id = ?"
	err := b.db.
		QueryRow(query, id).
		Scan(&user.ID, &user.FirstName, &user.LastName, &user.Username, &user.PhoneNumber, &user.IsConfirmed)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}

		return nil, err
	}

	return user, nil
}

func (b *SonateBot) SaveContact(contact *telebot.Contact) error {
	user, err := b.User(contact.UserID)
	if user != nil {
		return nil
	}

	query := "INSERT OR IGNORE INTO users (tg_id, first_name, last_name, username, phone_number, is_confirmed) VALUES (?, ?, ?, ?, ?, ?)"
	_, err = b.db.Exec(query, contact.UserID, contact.FirstName, contact.LastName, "", contact.PhoneNumber, false)
	if err != nil {
		return err
	}

	_, err = b.User(contact.UserID)
	return err
}

func (b *SonateBot) SetUserConfirmation(userID int64, isConfirmed bool) error {
	user, err := b.User(userID)
	if err != nil {
		return err
	}
	if user == nil {
		return fmt.Errorf("user not found with id=%d to set confirmation", userID)
	}

	query := "UPDATE users SET is_confirmed = ? WHERE tg_id = ?"
	if _, err = b.db.Exec(query, isConfirmed, userID); err != nil {
		return err
	}

	user.IsConfirmed = isConfirmed
	return nil
}

func (b *SonateBot) LoadPages() error {
	query := "SELECT id, code, title, items, is_public FROM pages"
	rows, err := b.db.Query(query)
	if err != nil {
		return err
	}
	defer rows.Close()

	pages := map[int]*Page{}
	for rows.Next() {
		var id int
		var code *string
		var title, itemsJSON string
		var isPublic bool
		err = rows.Scan(&id, &code, &title, &itemsJSON, &isPublic)
		if err != nil {
			return err
		}
		var pageItems []*PageItem
		if err = json.Unmarshal([]byte(itemsJSON), &pageItems); err != nil {
			return err
		}

		pages[id] = &Page{
			ID:       id,
			Code:     code,
			Title:    title,
			Items:    pageItems,
			IsPublic: isPublic,
		}
	}

	for _, page := range pages {
		for _, item := range page.Items {
			if item.ShowPageID != nil {
				if p, ok := pages[*item.ShowPageID]; ok {
					item.ShowPage = p
				} else {
					return fmt.Errorf("page not found with id=%d", *item.ShowPageID)
				}
			}
		}

		if page.Code != nil {
			switch *page.Code {
			case "PUBLIC":
				b.publicPage = page

			case "USER_HOME":
				b.userHomePage = page
			}
		}
	}

	b.pages = pages

	return nil
}
