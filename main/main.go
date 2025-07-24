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

func main() {
	cfg = config.LoadConfig("config.json")
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
		if update.Message == nil {
			continue
		}

	}
}
