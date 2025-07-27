package main

import (
	"config"
	"database/sql"
	"fmt"
	"log"
	"my_database"
	"sync"
	"tgbot"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	_ "github.com/mattn/go-sqlite3"
)

var DB my_database.DataBaseSites
var bot tgbot.TGBot
var cfg config.Config
var MU sync.Mutex

func isTableEmpty(tableName string) bool {
	var count int
	err := DB.DB.QueryRow(fmt.Sprintf("SELECT COUNT(*) FROM %s", tableName)).Scan(&count)
	if err != nil {
		log.Println(err)
		return false
	}
	return count == 0
}

func isAdmin(user *tgbotapi.User) bool {
	userID := user.ID
	var exists bool
	err := DB.DB.QueryRow("SELECT EXISTS(SELECT 1 FROM admins WHERE user_id = ?)", userID).Scan(&exists)
	if err != nil {
		log.Println(err)
	}
	return exists
}

func addAdmin(user *tgbotapi.User) {
	if isAdmin(user) {
		return
	}
	userID := user.ID
	_, err := DB.DB.Exec("INSERT INTO admins (user_id) VALUES (?)", userID)
	if err != nil {
		log.Println(err)
	}
}

func CatchMessage(update tgbotapi.Update) {
	MU.Lock()
	defer MU.Unlock()

	chat := update.Message.Chat
	user := update.Message.From

	if chat.Type == "private" {
		userID := user.ID
		var exists bool
		err := DB.DB.QueryRow("SELECT EXISTS(SELECT 1 FROM users WHERE user_id = ?)", userID).Scan(&exists)
		if err != nil {
			log.Println("Ошибка проверки пользователя:", err)
			return
		}

		if !exists {
			_, err := DB.DB.Exec("INSERT INTO users(user_id, user_group) VALUES (?, 0)", userID)
			if err != nil {
				log.Println("Ошибка добавления пользователя:", err)
			}
		}

		//first user -> admin
		if isTableEmpty("admins") {
			addAdmin(user)
		}
	}

	if chat.Type == "group" || chat.Type == "supergroup" {
		log.Println("Сообщение в группе %d от пользователя %d", chat.ID, user.ID)
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
