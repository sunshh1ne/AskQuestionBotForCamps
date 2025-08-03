package main

import (
	"config"
	"database/sql"
	"fmt"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	_ "github.com/mattn/go-sqlite3"
	"log"
	"my_database"
	"strconv"
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
	bot.SendMessage(update.Message.From.ID, "Oh no, La Police...", false)
}

func adminByLink(update tgbotapi.Update) bool {
	text := update.Message.CommandArguments()
	if DB.IsPassword(text) {
		err := DB.AddAdmin(update.Message.From.ID)
		if err != nil {
			bot.SendMessage(
				int(update.Message.Chat.ID),
				"❌ Ошибка: Не удалось выдать права администратора. "+
					"Подробнее: "+err.Error(), false,
			)
			return false
		}
		DB.DelPassword(text)
		return true
	}
	return false
}

func getLinkForAdmin() string {
	pass := DB.GetPassword(cfg.LenOfPass)
	url := "https://t.me/MoscowProgrammingTeam_bot?start=" + pass
	return url
}

func groupByLink(update tgbotapi.Update) (int64, bool, bool) {
	text := update.Message.CommandArguments()
	group := DB.GroupByKeyword(text)
	invitable := DB.IsInvitable(group)
	wasingroup := DB.WasInGroup(update, group)
	if group != -1 && (invitable || wasingroup) {
		DB.AddInGroup(update, group, wasingroup)
		//	добавили юзера к группе
	}
	return group, invitable, wasingroup
}

func needsNameRegistration(userID int) bool {
	var name, surname string
	err := DB.DB.QueryRow(
		"SELECT user_name, user_surname FROM users WHERE user_id = ?",
		userID,
	).Scan(&name, &surname)

	return err != nil || name == "" || surname == ""
}

func askForName(userID int) {
	bot.SendMessage(userID, "📝Введите ваше имя и фамилию через /s \nПример:\n /s Иван Иванов\n /s Анна Петрова",
		true)
}

func handleName(update tgbotapi.Update) {
	chatID := update.Message.From.ID
	text := update.Message.CommandArguments()
	parts := strings.SplitN(strings.TrimSpace(text), " ", 2)
	if len(parts) != 2 {
		bot.SendMessage(chatID, "❌ Неверный формат. Введите Имя и Фамилию через пробел", false)
		askForName(chatID)
		return
	}
	if err := DB.SaveUserName(update.Message.From.ID, parts[0], parts[1]); err != nil {
		bot.SendMessage(chatID, "⚠️ Ошибка сохранения. Попробуйте позже.", false)
		return
	}
	bot.SendMessage(
		int(update.Message.Chat.ID),
		fmt.Sprintf("✅ Спасибо, %s %s! Ваше имя сохранено.", parts[0], parts[1]), false)
}

func CatchPrivateCommand(update tgbotapi.Update) {
	command := update.Message.Command()
	if command == "start" {
		grouplink, invitable, wasingroup := groupByLink(update)
		if grouplink != -1 {
			if wasingroup {
				bot.SendMessage(update.Message.From.ID, "Теперь вы задаете вопросы в группу (другая группа, названия пришпилить надо)", false)
			} else if invitable {
				bot.SendMessage(update.Message.From.ID, "Поздравляю! Вы записаны в группу. Чтобы задать вопрос, напишите мне сообщение, я перешлю его преподавателям", false)
			} else {
				bot.SendMessage(update.Message.From.ID, "К сожалению, запись в данную группу уже закрыта. Сообщите преподавателю об этой проблеме", false)
			}
		}
		newAdmin := adminByLink(update)
		if newAdmin {
			bot.SendMessage(update.Message.From.ID, "Поздравляю! Вы получили права администратора.", false)
		} else {
			if needsNameRegistration(update.Message.From.ID) {
				askForName(update.Message.From.ID)
			}
		}
		return
	}
	if DB.IsBanned(update.Message.From.ID, DB.GetGroupByUser(update.Message.From.ID)) {
		bot.SendMessage(update.Message.From.ID, "❌ Вы заблокированы, обратитесь к преподавателю", false)
		return
	}
	switch command {
	case "getlink":
		if fl, err := DB.IsAdmin(update.Message.From.ID); err == nil && fl {
			bot.SendMessage(update.Message.From.ID, "Ссылка на добавление администратора: "+getLinkForAdmin(), false)
		} else {
			detectYoungHacker(update)
		}

	case "changename":
		askForName(update.Message.From.ID)

	case "s":
		handleName(update)
	}
}

func getUserIDByReply(update tgbotapi.Update) (int, error) {
	if update.Message.ReplyToMessage.ForwardFrom != nil {
		return update.Message.ReplyToMessage.ForwardFrom.ID, nil
	}
	msgID := update.Message.ReplyToMessage.MessageID
	return DB.GetUserIDByMsgIDInAdminChat(msgID)
}

func getUserIDByMsg(update tgbotapi.Update) (int, error) {
	text := update.Message.CommandArguments()
	userID, err := strconv.Atoi(text)
	return userID, err
}

func CatchGroupCommand(update tgbotapi.Update) {
	if fl, err := DB.IsAdmin(update.Message.From.ID); err == nil && !fl {
		bot.SendMessage(int(update.Message.Chat.ID), "God Damn!", false)
		detectYoungHacker(update)
		return
	}
	command := update.Message.Command()
	switch command {
	case "getlink":
		bot.SendMessage(int(update.Message.Chat.ID), "Ссылка для записи в группу: "+getLinkForUsers(update), false)

	case "getquestions":
		user_ids, admin_msg_ids, user_msg_ids, user_chat_ids, user_names, group_ids := DB.GetQuestions(cfg.CountOfQuestions, update.Message.Chat.ID)
		if len(user_chat_ids) == 0 {
			bot.SendMessage(int(update.Message.Chat.ID), "✅ Список неотвеченных вопросов пуст", false)
			return
		}
		bot.SendMessage(
			int(update.Message.Chat.ID),
			fmt.Sprintf("📬 *Неотвеченные вопросы \\(%d\\):*", len(user_chat_ids)), true,
		)

		for i := 0; i < len(user_chat_ids); i++ {
			bot.SendMessage(
				int(update.Message.Chat.ID),
				fmt.Sprintf("❓ *Вопрос от:* %s\n👤 *ID пользователя:* `%d`",
					user_names[i], user_ids[i]), true,
			)
			sent := bot.SendForward(group_ids[i],
				user_chat_ids[i],
				user_msg_ids[i])
			fmt.Println(sent)
			DB.SetNewAdminChatId(sent, admin_msg_ids[i])

			if i+1 < len(user_chat_ids) {
				bot.SendMessage(
					int(update.Message.Chat.ID),
					"────────────────", false,
				)
			}
		}

	case "stoplink":
		err := DB.StopGroupLink(update.Message.Chat.ID)
		if err != nil {
			bot.SendMessage(
				int(update.Message.Chat.ID),
				"❌ Ошибка приостановления приглашений в группу : "+err.Error(), false,
			)
			return
		}
		bot.SendMessage(int(update.Message.Chat.ID), "✅ Запись в группу успешно приостановлена.", false)

	case "contlink":
		err := DB.ContinueGroupLink(update.Message.Chat.ID)
		if err != nil {
			bot.SendMessage(
				int(update.Message.Chat.ID),
				"❌ Ошибка восстановления приглашений в группу : "+err.Error(), false,
			)
			return
		}
		bot.SendMessage(int(update.Message.Chat.ID), "✅ Запись в группу успешно восстановлена.", false)

	case "ban":
		var userID int
		var err error
		if update.Message.ReplyToMessage != nil {
			userID, err = getUserIDByReply(update)
			if err != nil {
				bot.SendMessage(
					int(update.Message.Chat.ID),
					"❌ Ошибка: Не удалось найти пользователя по ID сообщения. "+
						"Подробнее: "+err.Error(), false,
				)
				return
			}
		} else {
			userID, err = getUserIDByMsg(update)
			if err != nil {
				bot.SendMessage(
					int(update.Message.Chat.ID),
					"❌ Ошибка: Некорректный ID пользователя. "+
						"Используйте `/ban <ID>` или ответьте на сообщение. "+
						"Ошибка: "+err.Error(), false,
				)
				return
			}
		}
		if fl, err := DB.IsAdmin(userID); err == nil && fl {
			bot.SendMessage(
				int(update.Message.Chat.ID),
				"❌ Ошибка бана: нельзя забанить администратора", false,
			)
		}
		err = DB.BanUser(userID, update.Message.Chat.ID)
		if err != nil {
			bot.SendMessage(
				int(update.Message.Chat.ID),
				"❌ Ошибка бана: "+err.Error(), false,
			)
		} else {
			bot.SendMessage(
				int(update.Message.Chat.ID),
				fmt.Sprintf("✅ Пользователь `%d` забанен", userID), true,
			)
		}

	case "unban":
		text := update.Message.CommandArguments()
		userID, err := strconv.Atoi(text)
		if err != nil {
			bot.SendMessage(
				int(update.Message.Chat.ID),
				"❌ Ошибка: Некорректный ID пользователя. "+
					"Используйте /unban <ID>. "+
					"Ошибка: "+err.Error(), false,
			)
			return
		}
		err = DB.UnBanUser(userID, update.Message.Chat.ID)
		if err != nil {
			bot.SendMessage(
				int(update.Message.Chat.ID),
				"❌ Ошибка разбана: "+err.Error(), false,
			)
		} else {
			bot.SendMessage(
				int(update.Message.Chat.ID),
				fmt.Sprintf("✅ Пользователь `%d` разбанен", userID), true,
			)
		}

	case "delbannedq":
		cnt, err := DB.DeleteQuestionsByBannedUsers(update.Message.Chat.ID)
		if err != nil {
			bot.SendMessage(
				int(update.Message.Chat.ID),
				fmt.Sprintf("❌ Ошибка: Не удалось удалить все сообщения заблокированных пользователей\\. Удалено: %d сообщений"+
					"Подробнее: "+err.Error(), cnt), false,
			)
			return
		}
		bot.SendMessage(
			int(update.Message.Chat.ID),
			fmt.Sprintf("✅ Успешно удалены сообщения от всех пользователей: %d", cnt), false,
		)

	case "delq":
		if update.Message.ReplyToMessage == nil && len(update.Message.CommandArguments()) == 0 {
			cnt, err := DB.DeleteQuestionsByUsers(update.Message.Chat.ID)
			if err != nil {
				bot.SendMessage(
					int(update.Message.Chat.ID),
					fmt.Sprintf("❌ Ошибка: Не удалось удалить все сообщения пользователей\\. Удалено: %d сообщений"+
						"Подробнее: "+err.Error(), cnt), false,
				)
				return
			}
			bot.SendMessage(
				int(update.Message.Chat.ID),
				fmt.Sprintf("✅ Успешно удалены сообщения от всех пользователей: %d", cnt), false,
			)

		} else {
			var userID int
			var err error
			if update.Message.ReplyToMessage != nil {
				userID, err = getUserIDByReply(update)
				if err != nil {
					bot.SendMessage(
						int(update.Message.Chat.ID),
						"❌ Ошибка: Не удалось найти пользователя по ID сообщения. "+
							"Подробнее: "+err.Error(), false,
					)
					return
				}
			} else {
				userID, err = getUserIDByMsg(update)
				if err != nil {
					bot.SendMessage(
						int(update.Message.Chat.ID),
						"❌ Ошибка: Некорректный ID пользователя. "+
							"Используйте `/ban <ID>` или ответьте на сообщение. "+
							"Ошибка: "+err.Error(), false,
					)
					return
				}
			}
			cnt := DB.DeleteQuestionsByUser(userID)
			bot.SendMessage(
				int(update.Message.Chat.ID),
				fmt.Sprintf("✅ Успешно удалены все вопросы от пользователя `%d` \\(%d вопросов\\) ", userID, cnt), true,
			)
		}

	case "ask":

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
	sent := bot.SendForward(group, msg.Chat.ID, msg.MessageID)
	DB.AddQuestion(update, sent)
	bot.SendMessage(
		int(update.Message.Chat.ID),
		fmt.Sprintf("✅ Ваш вопрос отправлен предподавателям\\. ID вашего вопроса \\- `%d`", msg.MessageID), true,
	)
}

func replyAdmin(update tgbotapi.Update) {
	repliedMsg := update.Message.ReplyToMessage

	userChatID, exists := DB.GetUserChatIdByAdminChatId(*repliedMsg)
	if !exists {
		log.Println("Не найдено исходное сообщение пользователя")
		return
	}
	userMsgID, err := DB.GetUserMsgIDByAdminID(repliedMsg.MessageID)
	catchError(err)
	bot.SendReplyMessage(
		userChatID,
		fmt.Sprintf("✅ Ответ на ваш вопрос с ID `%d`", userMsgID), true, userMsgID,
	)
	bot.SendForward(int64(userChatID), update.Message.Chat.ID, update.Message.MessageID)

	bot.SendReplyMessage(
		int(update.Message.Chat.ID),
		fmt.Sprintf("✅ Ваш ответ отправлен пользователю с ID \\- `%d`", userChatID), true, update.Message.MessageID,
	)
	DB.DelQuestion(*repliedMsg)
}

func CatchPrivateMessage(update tgbotapi.Update) {
	if update.Message.IsCommand() {
		CatchPrivateCommand(update)
		return
	}
	if DB.IsBanned(update.Message.From.ID, DB.GetGroupByUser(update.Message.From.ID)) {
		bot.SendMessage(update.Message.From.ID, "❌ Вы заблокированы, обратитесь к преподавателю", false)
		return
	}

	if !DB.HasName(update.Message.From.ID) {
		askForName(update.Message.From.ID)
		return
	}
	group := DB.GetGroupByUser(update.Message.From.ID)
	if group == -1 {
		bot.SendMessage(update.Message.From.ID, "Вы не присоединены к группе. Обратитесь к преподавателю за ссылкой для вступления в группу.", false)
	} else {
		forwardToGroup(update, group)
	}
}

func CatchGroupMessage(update tgbotapi.Update) {
	//в группе ботом могут пользоваться только админы
	if fl, err := DB.IsAdmin(update.Message.From.ID); err == nil && !fl {
		detectYoungHacker(update)
		_, err := bot.Bot.LeaveChat(tgbotapi.ChatConfig{
			ChatID: update.Message.Chat.ID,
		})
		catchError(err)
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
		err := DB.AddAdmin(user.ID)
		if err != nil {
			bot.SendMessage(
				int(update.Message.Chat.ID),
				"❌ Ошибка: Не удалось выдать права администратора. "+
					"Подробнее: "+err.Error(), false,
			)
		}
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
