package tgbot

import (
	"log"
	"unicode/utf8"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
)

type TGBot struct {
	Bot *tgbotapi.BotAPI
}

func (tgbot *TGBot) Init(tgbotkey string) {
	bot, err := tgbotapi.NewBotAPI(tgbotkey)
	if err != nil {
		log.Fatal(err)
	}
	bot.Debug = true
	tgbot.Bot = bot
	log.Printf("Authorized on account %s", bot.Self.UserName)
}

func RemoveNonUTF8Runes(s string) string {
	var valid []rune
	for i, w := 0, 0; i < len(s); i += w {
		runeValue, width := utf8.DecodeRuneInString(s[i:])
		if runeValue != utf8.RuneError {
			valid = append(valid, runeValue)
		}
		w = width
	}
	return string(valid)
}

func (bot *TGBot) SendMessage(id int, message string, isMarkdown bool) tgbotapi.Message {
	msg := tgbotapi.NewMessage(int64(id), message)
	if isMarkdown {
		msg.ParseMode = "MarkdownV2"
	}
	sentMsg, err := bot.Bot.Send(msg)
	if err != nil {
		log.Println(err)
	}
	return sentMsg
}

func (bot *TGBot) SendReplyMessage(id int, message string, isMarkdown bool, replyID int) tgbotapi.Message {
	msg := tgbotapi.NewMessage(int64(id), message)
	if isMarkdown {
		msg.ParseMode = "MarkdownV2"
	}
	msg.ReplyToMessageID = replyID
	sentMsg, err := bot.Bot.Send(msg)
	if err != nil {
		log.Println(err)
	}
	return sentMsg
}

func (bot *TGBot) SendForward(id1, id2 int64, id3 int) tgbotapi.Message {
	msg, err := bot.Bot.Send(tgbotapi.NewForward(id1, id2, id3))
	if err != nil {
		log.Println(err)
	}
	return msg
}
