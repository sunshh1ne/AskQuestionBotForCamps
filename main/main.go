package main

import (
	"config"
	"database/sql"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	_ "github.com/mattn/go-sqlite3"
	"log"
	"my_database"
	"sync"
	"tgbot"
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
	if !DB.IsAdmin(update.Message.From) {
		bot.SendMessage(int(update.Message.Chat.ID), "God Damn!")
		detectYoungHacker(update)
		return
	}
	command := update.Message.Command()
	switch command {
	case "getlink":
		bot.SendMessage(int(update.Message.Chat.ID), "Ссылка для записи в группу: "+getLinkForUsers(update))
	case "getquestions":
		user_ids, admin_msg_ids, user_msg_ids, user_chat_ids := DB.GetQuestions(cfg.CountOfQuestions)
		for i := 0; i < len(user_chat_ids); i++ {
			user_msg_id := user_msg_ids[i]
			admin_msg_id := admin_msg_ids[i]
			user_chat_id := user_chat_ids[i]
			user_id := user_ids[i]
			//Допилить нормальный(красивый) вывод сообщения
			sent, err := bot.Bot.Send(tgbotapi.NewForward(int64(DB.GetGroup(user_id)), user_chat_id, user_msg_id))
			catchError(err)
			DB.SetNewAdminChatId(sent, admin_msg_id)
		}
	}
}

func getLinkForUsers(update tgbotapi.Update) string {
	keyword := DB.GetKeyword(update)
	url := "https://t.me/MoscowProgrammingTeam_bot?start=" + keyword
	return url
}

func addInNewGroup(update tgbotapi.Update) {
	DB.NewChat(update)
}

func forwardToGroup(update tgbotapi.Update, group int64) {
	msg := update.Message
	sent, err := bot.Bot.Send(tgbotapi.NewForward(group, msg.Chat.ID, msg.MessageID))
	catchError(err)
	DB.AddQuestion(update, sent)
}

func replyAdmin(update tgbotapi.Update) {
	repliedMsg := update.Message.ReplyToMessage

	userChatID, exists := DB.GetUserChatIdByAdminChatId(*repliedMsg)
	if !exists {
		log.Println("Не найдено исходное сообщение пользователя")
		return
	}

	_, err := bot.Bot.Send(tgbotapi.NewForward(int64(userChatID), update.Message.Chat.ID, update.Message.MessageID))
	catchError(err)

	DB.DelQuestion(*repliedMsg)
}

func CatchPrivateMessage(update tgbotapi.Update) {
	if update.Message.IsCommand() {
		CatchPrivateCommand(update)
	} else {
		group := DB.GetGroup(update.Message.Chat.ID)
		if group == -1 {
			bot.SendMessage(update.Message.From.ID, "Вы не присоединены к группе. Обратитесь к преподавателю за ссылкой для вступления в группу.")
		} else {
			forwardToGroup(update, int64(group))
		}
	}
}

func CatchGroupMessage(update tgbotapi.Update) {
	//в группе ботом могут пользоваться только админы
	if !DB.IsAdmin(update.Message.From) {
		detectYoungHacker(update)
		_, err := bot.Bot.LeaveChat(tgbotapi.ChatConfig{
			ChatID: update.Message.Chat.ID,
		})
		catchError(err)
		detectYoungHacker(update)
		return
	}

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
		if update.Message.ReplyToMessage != nil && update.Message.ReplyToMessage.From.IsBot {
			replyAdmin(update)
		}
	}
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
		CatchPrivateMessage(update)
	}

	if chat.Type == "group" || chat.Type == "supergroup" {
		CatchGroupMessage(update)
	}
}

func CatchCallbackQuery(update tgbotapi.Update) {

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
		} else if update.CallbackQuery != nil {
			CatchCallbackQuery(update)
		}
	}
}
