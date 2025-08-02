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
			user_id      INTEGER PRIMARY KEY
								 UNIQUE
								 NOT NULL,
			user_group   INTEGER,
			user_name    TEXT,
			user_surname TEXT
		);`,
		`CREATE TABLE IF NOT EXISTS passwords (
    		password TEXT
    	);`,
		`CREATE TABLE IF NOT EXISTS not_answered_questions (
			user_id      INTEGER,
			admin_msg_id INTEGER,
			user_msg_id  INTEGER,
			user_chat_id
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

func (DB *DataBaseSites) NewChat(update tgbotapi.Update) {
	keyword := random.GetRandom(20)
	_, err := DB.DB.Exec("INSERT INTO chats(chat_id, keyword) VALUES (?, ?);", update.Message.Chat.ID, keyword)
	if err != nil {
		log.Println(err)
	}
}

func (DB *DataBaseSites) GetKeyword(update tgbotapi.Update) string {
	chatID := update.Message.Chat.ID
	var keyword string
	err := DB.DB.QueryRow("SELECT keyword FROM chats WHERE chat_id = ?", chatID).Scan(&keyword)
	if err != nil {
		log.Println(err)
	}
	return keyword
}

func (DB *DataBaseSites) GroupByKeyword(text string) int {
	var chatID int
	err := DB.DB.QueryRow("SELECT chat_id FROM chats WHERE keyword = ?", text).Scan(&chatID)
	if err != nil {
		if err == sql.ErrNoRows {
			return -1
		}
		log.Println(err)
	}
	return chatID
}

func (DB *DataBaseSites) AddInGroup(update tgbotapi.Update, newGroup int) {
	userID := update.Message.Chat.ID
	_, err := DB.DB.Exec("UPDATE users SET user_group = ? WHERE user_id = ?", newGroup, userID)
	if err != nil {
		log.Println(err)
	}
}

func (DB *DataBaseSites) GetGroup(userID int64) int {
	var group int
	err := DB.DB.QueryRow("SELECT user_group FROM users WHERE user_id = ?", userID).Scan(&group)
	if err != nil {
		log.Println(err)
	}
	return group
}

func (DB *DataBaseSites) AddQuestion(update tgbotapi.Update, reply tgbotapi.Message) {
	//	update - сообщение которое бот получил
	//	reply - сообщение которое бот переслал в чат (копия, ответ на которую мы ждем)
	_, err := DB.DB.Exec("INSERT INTO not_answered_questions(user_id, admin_msg_id, user_msg_id, user_chat_id) VALUES (?, ?, ?, ?);",
		update.Message.From.ID, reply.MessageID, update.Message.MessageID, update.Message.Chat.ID)
	if err != nil {
		log.Println(err)
	}
}

func (DB *DataBaseSites) GetUserChatIdByAdminChatId(msg tgbotapi.Message) (int, bool) {
	adminID := msg.MessageID
	var userID int
	err := DB.DB.QueryRow("SELECT user_id FROM not_answered_questions WHERE admin_msg_id = ?", adminID).Scan(&userID)
	if err != nil {
		if err == sql.ErrNoRows {
			return 0, false
		}
		log.Println(err)
	}
	return userID, true
}

func (DB *DataBaseSites) DelQuestion(msg tgbotapi.Message) {
	_, err := DB.DB.Exec("DELETE FROM not_answered_questions WHERE admin_msg_id = ?", msg.MessageID)
	if err != nil {
		log.Println(err)
	}
}

func (DB *DataBaseSites) GetQuestions(cnt int) ([]int64, []int, []int, []int64, []string) {
	rows, err := DB.DB.Query(`
        SELECT q.user_id, q.admin_msg_id, q.user_msg_id, q.user_chat_id, 
               COALESCE(u.user_name || ' ' || u.user_surname, 'Аноним') as user_name
        FROM not_answered_questions q
        LEFT JOIN users u ON q.user_id = u.user_id
        LIMIT ?`, cnt)
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	var user_ids, user_chat_ids []int64
	var user_msg_ids, admin_msg_ids []int
	var user_names []string
	for rows.Next() {
		var user_msg_id, admin_msg_id int
		var user_id, user_chat_id int64
		var user_name string
		err := rows.Scan(&user_id, &admin_msg_id, &user_msg_id, &user_chat_id, &user_name)
		if err != nil {
			log.Println(err)
		}
		user_msg_ids = append(user_msg_ids, user_msg_id)
		admin_msg_ids = append(admin_msg_ids, admin_msg_id)
		user_chat_ids = append(user_chat_ids, user_chat_id)
		user_ids = append(user_ids, user_id)
		user_names = append(user_names, user_name)
	}
	return user_ids, admin_msg_ids, user_msg_ids, user_chat_ids, user_names
}

func (DB *DataBaseSites) SetNewAdminChatId(nmsg tgbotapi.Message, oldid int) {
	nID := nmsg.MessageID
	_, err := DB.DB.Exec("UPDATE not_answered_questions SET admin_msg_id = ? WHERE admin_msg_id = ?", nID, oldid)
	if err != nil {
		log.Println(err)
	}
}

func (DB *DataBaseSites) SaveUserName(userID int64, name, surname string) error {
	_, err := DB.DB.Exec(`
        UPDATE users 
        SET user_name = ?, user_surname = ?
        WHERE user_id = ?`,
		name, surname, userID,
	)
	return err
}

func (DB *DataBaseSites) HasName(userID int64) bool {
	var name string
	err := DB.DB.QueryRow("SELECT user_name FROM users WHERE user_id = ?", userID).Scan(&name)
	return err == nil && name != ""
}
