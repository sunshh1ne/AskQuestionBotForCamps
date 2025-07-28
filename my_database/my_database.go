package my_database

import (
	"database/sql"
	"fmt"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	_ "github.com/mattn/go-sqlite3"
	"log"
	"random"
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

func (DB *DataBaseSites) IsAdmin(user *tgbotapi.User) bool {
	userID := user.ID
	var exists bool
	err := DB.DB.QueryRow("SELECT EXISTS(SELECT 1 FROM admins WHERE user_id = ?)", userID).Scan(&exists)
	if err != nil {
		log.Println(err)
	}
	return exists
}

func (DB *DataBaseSites) AddAdmin(user *tgbotapi.User) {
	if DB.IsAdmin(user) {
		return
	}
	userID := user.ID
	_, err := DB.DB.Exec("INSERT INTO admins (user_id) VALUES (?)", userID)
	if err != nil {
		log.Println(err)
	}
}

func (DB *DataBaseSites) IsPassword(text string) bool {
	var exists bool
	err := DB.DB.QueryRow("SELECT EXISTS(SELECT 1 FROM passwords WHERE password = ?)", text).Scan(&exists)
	if err != nil {
		log.Println(err)
	}
	return exists
}

func (DB *DataBaseSites) DelPassword(text string) {
	_, err := DB.DB.Exec("DELETE FROM passwords WHERE password = ?", text)
	if err != nil {
		log.Println(err)
	}
}

func (DB *DataBaseSites) IsTableEmpty(tableName string) bool {
	var count int
	err := DB.DB.QueryRow(fmt.Sprintf("SELECT COUNT(*) FROM %s", tableName)).Scan(&count)
	if err != nil {
		log.Println(err)
		return false
	}
	return count == 0
}

func (DB *DataBaseSites) GetPassword(len int) string {
	password := random.GetRandom(len)
	_, err := DB.DB.Exec("INSERT INTO passwords(password) VALUES (?);", password)
	if err != nil {
		log.Println(err)
	}
	return password
}
