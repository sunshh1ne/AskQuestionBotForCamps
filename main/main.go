package main

import (
	"config"
	"database/sql"
	"log"
	"my_database"
	"tgbot"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	_ "github.com/mattn/go-sqlite3"
)

var DB my_database.DataBaseSites
var bot tgbot.TGBot
var cfg config.Config

func CatchMessage(update tgbotapi.Update) {
	user_id := update.Message.From.ID
	var exists bool
	err := DB.DB.QueryRow("SELECT EXISTS(SELECT 1 FROM users WHERE user_id = ?);", user_id).Scan(&exists)
	if err != nil {
		log.Fatal(err)
	}
	if !exists {
		_, err := DB.DB.Exec("INSERT INTO users(user_id, user_group) VALUES (?, 0);", user_id)
		if err != nil {
			log.Fatal(err)
		}
	}

	if update.Message.Text == cfg.Keyword {
		_, err := DB.DB.Exec("INSERT OR IGNORE INTO admins(user_id) VALUES (?);", user_id)
		if err != nil {
			log.Fatal(err)
		}
		return
	}
}

func main() {
	cfg = config.LoadConfig("D:\\moscowsbornaya\\config.json")
	DB.Init()
	defer func(DB *sql.DB) {
		err := DB.Close()
		if err != nil {
			log.Fatal(err)
		}
	}(DB.DB)
	log.Println("Connected to database")

	bot.Init(cfg.TGBotKey)
	u := tgbotapi.NewUpdate(0)

	updates, err := bot.Bot.GetUpdatesChan(u)
	if err != nil {
		log.Fatal(err)
	}

	for update := range updates {
		if update.Message != nil {
			CatchMessage(update)
		} else if update.CallbackQuery != nil {

		}
	}
}
