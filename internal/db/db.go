package db

import (
	"database/sql"
	"log"

	_ "github.com/glebarez/go-sqlite"
)

func Init(path string) *sql.DB {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		log.Fatal(err)
	}

	// Init schema
	_, err = db.Exec(`
        CREATE TABLE IF NOT EXISTS users (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            tg_id BIGINT UNIQUE,
            phone TEXT,
            is_confirmed BOOLEAN DEFAULT 0,
            last_submission_at DATETIME,
		    created_at DATETIME NOT NULL
        );
    `)
	if err != nil {
		log.Fatal(err)
	}

	_, err = db.Exec(`
        CREATE TABLE IF NOT EXISTS schedules (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            cron_expr TEXT NOT NULL,
            message TEXT,
		    forward_message_id BIGINT,
            is_active BOOLEAN DEFAULT 1
        );
    `)
	if err != nil {
		log.Fatal(err)
	}

	_, err = db.Exec(`
        CREATE TABLE IF NOT EXISTS deliveries (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            schedule_id INTEGER,
            user_id BIGINT,
            status TEXT,
            attempts INTEGER DEFAULT 0,
            last_error TEXT,
            updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
            FOREIGN KEY(schedule_id) REFERENCES schedules(id)
        );
    `)
	if err != nil {
		log.Fatal(err)
	}

	return db
}
