package main

import (
	"config"
	"database/sql"
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

func catchError(err error) {
	if err != nil {
		log.Println(err)
	}
}

func detectYoungHacker(update tgbotapi.Update) {
	bot.SendMessage(update.Message.From.ID, "Oh no, La Police...")
}

func adminByLink(update tgbotapi.Update) bool {
	text := update.Message.CommandArguments()
	if DB.IsPassword(text) {
		DB.AddAdmin(update.Message.From)
		DB.DelPassword(text)
		return true
	}
	return false
}

func GetLinkForAdmin() string {
	pass := DB.GetPassword(cfg.LenOfPass)
	url := "https://t.me/MoscowProgrammingTeam_bot?start=" + pass
	return url
}

func groupByLink(update tgbotapi.Update) int {
	text := update.Message.CommandArguments()
	group := DB.GroupByKeyword(text)
	if group != -1 {
		DB.AddInGroup(update, group)
		//	добавили юзера к группе
	}
	return group
}

func CatchPrivateCommand(update tgbotapi.Update) {
	command := update.Message.Command()
	switch command {
	case "start":
		newAdmin := adminByLink(update)
		if newAdmin {
			bot.SendMessage(update.Message.From.ID, "Поздравляю! Вы получили права администратора.")
		}
		grouplink := groupByLink(update)
		if grouplink != -1 {
			bot.SendMessage(update.Message.From.ID, "Поздравляю! Вы записаны в группу. Чтобы задать вопрос, напишите мне сообщение, я перешлю его преподавателям")
		}

	case "getlink":
		if DB.IsAdmin(update.Message.From) {
			bot.SendMessage(update.Message.From.ID, "Ссылка на добавление администратора: "+GetLinkForAdmin())
		} else {
			bot.SendMessage(update.Message.From.ID, "Спички детям не игрушка!")
			detectYoungHacker(update)
		}
	}

}

func CatchGroupCommand(update tgbotapi.Update) {
	command := update.Message.Command()
	switch command {
	case "getlink":
		if DB.IsAdmin(update.Message.From) {
			bot.SendMessage(int(update.Message.Chat.ID), "Ссылка для записи в группу: "+getLinkForUsers(update))
		} else {
			bot.SendMessage(int(update.Message.Chat.ID), "Спички детям не игрушка!")
			detectYoungHacker(update)
		}
	}
}

func getLinkForUsers(update tgbotapi.Update) string {
	keyword := DB.GetKeyword(update)
	url := "https://t.me/MoscowProgrammingTeam_bot?start=" + keyword
	return url
}

func addInNewGroup(update tgbotapi.Update) {
	if !DB.IsAdmin(update.Message.From) {
		detectYoungHacker(update)
		_, err := bot.Bot.LeaveChat(tgbotapi.ChatConfig{
			ChatID: update.Message.Chat.ID,
		})
		catchError(err)
		return
	}
	DB.NewChat(update)
}

func forwardToGroup(update tgbotapi.Update, group int64) {
	msg := update.Message
	forward := tgbotapi.NewForward(group, msg.Chat.ID, msg.MessageID)
	sent, err := bot.Bot.Send(forward)
	if err != nil {
		log.Printf("Ошибка пересылки: %v", err)
	}
	DB.AddQuestion(update, sent)
}

func CatchMessage(update tgbotapi.Update) {
	MU.Lock()
	defer MU.Unlock()

	chat := update.Message.Chat
	user := update.Message.From

	userID := user.ID
	var exists bool
	err := DB.DB.QueryRow("SELECT EXISTS(SELECT 1 FROM users WHERE user_id = ?)", userID).Scan(&exists)
	catchError(err)

	if !exists {
		_, err := DB.DB.Exec("INSERT INTO users(user_id, user_group) VALUES (?, -1)", userID)
		catchError(err)
	}

	//first user -> admin
	if DB.IsTableEmpty("admins") {
		DB.AddAdmin(user)
	}

	if chat.Type == "private" {
		if update.Message.IsCommand() {
			CatchPrivateCommand(update)
		} else {
			group := DB.GetGroup(update)
			if group == -1 {
				bot.SendMessage(update.Message.From.ID, "Вы не присоединены к группе. Обратитесь к преподавателю за ссылкой для вступления в группу.")
			} else {
				forwardToGroup(update, int64(group))
			}
		}
	}

	if chat.Type == "group" || chat.Type == "supergroup" {
		if update.Message.NewChatMembers != nil {
			for _, member := range *update.Message.NewChatMembers {
				if member.ID == bot.Bot.Self.ID {
					addInNewGroup(update)
				}
			}
		}

		if update.Message.IsCommand() {
			CatchGroupCommand(update)
		} else {

		}
	}
}

func main() {
	cfg = config.LoadConfig("D:\\moscowsbornaya\\config.json")
	DB.Init()
	defer func(DB *sql.DB) {
		err := DB.Close()
		catchError(err)
	}(DB.DB)
	log.Println("Connected to database")

	bot.Init(cfg.TGBotKey)
	u := tgbotapi.NewUpdate(0)

	updates, err := bot.Bot.GetUpdatesChan(u)
	catchError(err)

	for update := range updates {
		if update.Message != nil {
			CatchMessage(update)
		} else {

		}
	}
}
