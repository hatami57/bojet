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

	_, err = db.Exec(`
        CREATE TABLE IF NOT EXISTS users (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            tg_id BIGINT UNIQUE,
            first_name TEXT,
            last_name TEXT,
            username TEXT,
            phone_number TEXT,
            is_confirmed BOOLEAN DEFAULT 0,
            last_submission_at DATETIME,
		    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
        );

		CREATE TABLE IF NOT EXISTS schedules (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            cron_expr TEXT NOT NULL,
            message TEXT,
		    forward_message_id BIGINT,
            is_active BOOLEAN DEFAULT 1
        );

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

		CREATE TABLE IF NOT EXISTS pages (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            title TEXT NOT NULL,
            items TEXT CHECK(items IS NULL OR json_valid(items)),
            is_public BOOLEAN DEFAULT 1
        );
    `)
	if err != nil {
		log.Fatal(err)
	}

	return db
}
