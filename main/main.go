package main

import (
	"config"
	"database/sql"
	"fmt"
	"log"
	"my_database"
	"password"
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

func isPassword(text string) bool {
	var exists bool
	err := DB.DB.QueryRow("SELECT EXISTS(SELECT 1 FROM passwords WHERE password = ?)", text).Scan(&exists)
	if err != nil {
		log.Println(err)
	}
	return exists
}

func delPassword(text string) {
	_, err := DB.DB.Exec("DELETE FROM passwords WHERE password = ?", text)
	if err != nil {
		log.Println(err)
	}
}

func adminByLink(update tgbotapi.Update) bool {
	text := update.Message.CommandArguments()
	if isPassword(text) {
		addAdmin(update.Message.From)
		delPassword(text)
		return true
	}
	return false
}

func GetLink() string {
	pass := password.GetPassword(DB.DB, cfg.LenOfPass)
	url := "https://t.me/MoscowProgrammingTeam_bot?start=" + pass
	return url
}

func CatchPrivateCommand(update tgbotapi.Update) {
	command := update.Message.Command()
	switch command {
	case "start":
		newAdmin := adminByLink(update)
		if newAdmin {

		} else {

		}
	case "getlink":
		if isAdmin(update.Message.From) {
			msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Ссылка на добавление администратора: "+GetLink())
			_, err := bot.Bot.Send(msg)
			if err != nil {
				log.Println(err)
			}
		} else {
			msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Спички детям не игрушка!")
			_, err := bot.Bot.Send(msg)
			if err != nil {
				log.Println(err)
			}
		}
	}

}

func CatchGroupCommand(update tgbotapi.Update) {

}

func CatchMessage(update tgbotapi.Update) {
	MU.Lock()
	defer MU.Unlock()

	chat := update.Message.Chat
	user := update.Message.From

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

	if chat.Type == "private" {
		if update.Message.IsCommand() {
			CatchPrivateCommand(update)
			return
		}

	}

	if chat.Type == "group" || chat.Type == "supergroup" {
		if update.Message.IsCommand() {
			CatchGroupCommand(update)
			return
		}

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
