package bot

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	"github.com/robfig/cron/v3"
	"gopkg.in/telebot.v4"
)

const maxRetries = 5

func (b *SonateBot) StartSchedulers() {
	c := cron.New()

	// Load schedules from DB
	rows, err := b.db.Query("SELECT id, cron_expr, message FROM schedules WHERE is_active=1")
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	for rows.Next() {
		var id int
		var expr, msg string
		rows.Scan(&id, &expr, &msg)

		scheduleID := id
		content := msg

		_, err := c.AddFunc(expr, func() {
			b.queueBroadcast(scheduleID, content)
		})
		if err != nil {
			log.Printf("Invalid cron expr %s: %v", expr, err)
		} else {
			log.Printf("Loaded schedule %d (%s)", id, expr)
		}
	}

	// Check inactive users daily at 09:00
	c.AddFunc("0 9 * * *", func() { b.checkInactiveUsers() })

	c.Start()
}

func (b *SonateBot) queueBroadcast(scheduleID int, msg string) {
	rows, err := b.db.Query("SELECT tg_id FROM users WHERE is_confirmed=1")
	if err != nil {
		log.Println("DB error:", err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var userID int64
		rows.Scan(&userID)

		go b.sendWithRetry(scheduleID, userID, msg)
	}
}

func (b *SonateBot) sendWithRetry(scheduleID int, userID int64, msg string) {
	delay := time.Second
	var err error

	for attempt := 1; attempt <= maxRetries; attempt++ {
		_, err = b.TgBot.Send(&telebot.User{ID: userID}, msg)
		if err == nil {
			_, _ = b.db.Exec(`UPDATE deliveries 
                            SET status='sent', attempts=? 
                            WHERE schedule_id=? AND user_id=?`,
				attempt, scheduleID, userID)
			log.Printf("✅ Sent to %d (attempt %d)", userID, attempt)
			return
		}

		time.Sleep(delay)
		delay *= 2
	}

	_, _ = b.db.Exec(`UPDATE deliveries 
                    SET status='failed' 
                    WHERE schedule_id=? AND user_id=?`,
		scheduleID, userID)

	log.Printf("❌ Giving up on user %d (schedule %d)", userID, scheduleID)
}

// Check for inactive users (no submission in 21 days)
func (b *SonateBot) checkInactiveUsers() {
	rows, err := b.db.Query(`
        SELECT tg_id, phone, last_submission_at
        FROM users 
        WHERE is_confirmed=1
          AND (last_submission_at IS NULL OR last_submission_at <= datetime('now','-21 days'))
    `)
	if err != nil {
		log.Println("DB error:", err)
		return
	}
	defer rows.Close()

	var message string
	for rows.Next() {
		var tgID int64
		var phone, last sql.NullString
		rows.Scan(&tgID, &phone, &last)

		message += fmt.Sprintf("User %d (%s) last submitted at: %s\n",
			tgID, phone.String, last.String)
	}

	if message != "" {
		for adminID := range b.cfg.AdminIDs {
			b.TgBot.Send(&telebot.User{ID: adminID}, "⚠️ Inactive users (3+ weeks):\n\n"+message)
		}
	}
}
