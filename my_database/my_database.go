package my_database

import (
	"database/sql"
	"log"

	_ "github.com/mattn/go-sqlite3"
)

type DataBaseSites struct {
	DB *sql.DB
}

func (dbs *DataBaseSites) Init() {
	DB, err := sql.Open("sqlite3", "database.db")
	if err != nil {
		log.Fatal(err)
	}
	dbs.DB = DB
	createTables(dbs.DB)
}

func createTables(db *sql.DB) {
	tables := []string{
		`CREATE TABLE IF NOT EXISTS admins (
    		user_id INTEGER PRIMARY KEY
		);`,
		`CREATE TABLE IF NOT EXISTS chats (
    		chat_id INTEGER PRIMARY KEY,
    		keyword TEXT
		);`,
		`CREATE TABLE IF NOT EXISTS keys_to_join (
		    [group] INTEGER,
    		key     TEXT
		);`,
		`CREATE TABLE IF NOT EXISTS users (
   			 user_id    INTEGER PRIMARY KEY
                       			UNIQUE
                       			NOT NULL,
    		user_group INTEGER
		);`,
		`CREATE TABLE IF NOT EXISTS passwords (
    		password TEXT
    	);`,
	}

	for _, tableSQL := range tables {
		_, err := db.Exec(tableSQL)
		if err != nil {
			log.Fatalf("Ошибка при создании таблицы: %v\nSQL: %s", err, tableSQL)
		}
	}
}
