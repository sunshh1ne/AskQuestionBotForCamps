package main

import (
	"config"
	"database/sql"
	"fmt"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	_ "github.com/mattn/go-sqlite3"
	"log"
	"my_database"
	"strings"
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

func needsNameRegistration(userID int) bool {
	var name, surname string
	err := DB.DB.QueryRow(
		"SELECT user_name, user_surname FROM users WHERE user_id = ?",
		userID,
	).Scan(&name, &surname)

	return err != nil || name == "" || surname == ""
}

func askForName(chatID int64) {
	msg := tgbotapi.NewMessage(chatID, `📝 *Введите ваше имя и фамилию через /s:*
Пример:
/s _Иван Иванов_
/s _Анна Петрова_`)
	msg.ParseMode = "Markdown"

	_, err := bot.Bot.Send(msg)
	catchError(err)
}

func handleName(update tgbotapi.Update) {
	chatID := update.Message.Chat.ID
	text := update.Message.CommandArguments()
	parts := strings.SplitN(strings.TrimSpace(text), " ", 2)
	if len(parts) != 2 {
		bot.SendMessage(int(chatID), "❌ Неверный формат. Введите Имя и Фамилию через пробел")
		askForName(chatID)
		return
	}
	if err := DB.SaveUserName(int64(update.Message.From.ID), parts[0], parts[1]); err != nil {
		bot.SendMessage(int(chatID), "⚠️ Ошибка сохранения. Попробуйте позже.")
		return
	}
	msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("✅ Спасибо, %s %s!", parts[0], parts[1]))
	_, err := bot.Bot.Send(msg)
	catchError(err)
}

func CatchPrivateCommand(update tgbotapi.Update) {
	command := update.Message.Command()
	switch command {
	case "start":
		grouplink := groupByLink(update)
		if grouplink != -1 {
			bot.SendMessage(update.Message.From.ID, "Поздравляю! Вы записаны в группу. Чтобы задать вопрос, напишите мне сообщение, я перешлю его преподавателям")
		}
		newAdmin := adminByLink(update)
		if newAdmin {
			bot.SendMessage(update.Message.From.ID, "Поздравляю! Вы получили права администратора.")
		} else {
			if needsNameRegistration(update.Message.From.ID) {
				askForName(update.Message.Chat.ID)
			}
		}

	case "getlink":
		if DB.IsAdmin(update.Message.From) {
			bot.SendMessage(update.Message.From.ID, "Ссылка на добавление администратора: "+GetLinkForAdmin())
		} else {
			bot.SendMessage(update.Message.From.ID, "Okak!")
			detectYoungHacker(update)
		}

	case "changename":
		askForName(update.Message.Chat.ID)

	case "s":
		handleName(update)
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
		user_ids, admin_msg_ids, user_msg_ids, user_chat_ids, user_names := DB.GetQuestions(cfg.CountOfQuestions)
		if len(user_chat_ids) == 0 {
			bot.SendMessage(int(update.Message.Chat.ID), "Список неотвеченных вопросов пуст")
			return
		}
		headerMsg := tgbotapi.NewMessage(update.Message.Chat.ID,
			fmt.Sprintf("📬 *Неотвеченные вопросы (%d):*", len(user_chat_ids)))
		headerMsg.ParseMode = "Markdown"
		_, err := bot.Bot.Send(headerMsg)
		catchError(err)

		for i := 0; i < len(user_chat_ids); i++ {
			infoMsg := tgbotapi.NewMessage(
				update.Message.Chat.ID,
				fmt.Sprintf("❓ *Вопрос от:* %s\n👤 *ID пользователя:* %d",
					user_names[i], user_ids[i]),
			)
			infoMsg.ParseMode = "Markdown"
			_, err := bot.Bot.Send(infoMsg)
			catchError(err)

			sent, err := bot.Bot.Send(tgbotapi.NewForward(
				int64(DB.GetGroup(user_ids[i])),
				user_chat_ids[i],
				user_msg_ids[i],
			))
			catchError(err)
			DB.SetNewAdminChatId(sent, admin_msg_ids[i])

			if i+1 < len(user_chat_ids) {
				_, err := bot.Bot.Send(tgbotapi.NewMessage(
					update.Message.Chat.ID,
					"────────────────",
				))
				catchError(err)
			}
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
		if !DB.HasName(update.Message.Chat.ID) {
			askForName(update.Message.Chat.ID)
			return
		}
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

func CatchGroupCallbackQuery(update tgbotapi.Update) {

}

func CatchPrivateCallbackQuery(update tgbotapi.Update) {

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
