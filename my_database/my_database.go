package my_database

import (
	"database/sql"
	"fmt"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	_ "github.com/mattn/go-sqlite3"
	"log"
	"math"
	"random"
	"strconv"
	"strings"
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
			chat_id   INTEGER PRIMARY KEY,
			keyword   TEXT,
			invitable INTEGER DEFAULT (1) 
		);`,
		`CREATE TABLE IF NOT EXISTS users (
			user_id         INTEGER PRIMARY KEY
									UNIQUE
									NOT NULL,
			user_group      INTEGER,
			user_name       TEXT,
			user_surname    TEXT,
			user_all_groups TEXT,
			banned          INTEGER DEFAULT (0),
        	username        TEXT
		);`,
		`CREATE TABLE IF NOT EXISTS passwords (
    		password TEXT
    	);`,
		`CREATE TABLE IF NOT EXISTS not_answered_questions (
			user_id      INTEGER,
			admin_msg_id INTEGER,
			user_msg_id  INTEGER,
			user_chat_id,
			user_group     INTEGER
		);`,
		`CREATE TABLE IF NOT EXISTS banned (
			user_id INTEGER,
			user_group INTEGER
		);`,
	}

	for _, tableSQL := range tables {
		_, err := db.Exec(tableSQL)
		if err != nil {
			log.Fatalf("Ошибка при создании таблицы: %v\nSQL: %s", err, tableSQL)
		}
	}
}

func (DB *DataBaseSites) IsAdmin(userID int) (bool, error) {
	var exists bool
	err := DB.DB.QueryRow("SELECT EXISTS(SELECT 1 FROM admins WHERE user_id = ?)", userID).Scan(&exists)
	return exists, err
}

func (DB *DataBaseSites) AddAdmin(userID int) error {
	if fl, err := DB.IsAdmin(userID); err != nil && fl {
		return nil
	}
	_, err := DB.DB.Exec("INSERT INTO admins (user_id) VALUES (?)", userID)
	return err
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

func (DB *DataBaseSites) NewChat(update tgbotapi.Update) error {
	var exists bool
	err := DB.DB.QueryRow(`
        SELECT EXISTS(SELECT 1 FROM chats WHERE chat_id = ?)`,
		update.Message.Chat.ID).Scan(&exists)
	if err != nil {
		return fmt.Errorf("failed to check chat existence: %v", err)
	}

	if exists {
		return nil
	}

	tx, err := DB.DB.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %v", err)
	}

	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	keyword := random.GetRandom(20)
	_, err = tx.Exec("INSERT INTO chats(chat_id, keyword) VALUES (?, ?)",
		update.Message.Chat.ID, keyword)
	if err != nil {
		return fmt.Errorf("failed to insert chat: %v", err)
	}

	tableName := fmt.Sprintf("questions_%d", int64(math.Abs(float64(update.Message.Chat.ID))))
	_, err = tx.Exec(fmt.Sprintf(`
        CREATE TABLE IF NOT EXISTS %s (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            admin_msg_id INTEGER NOT NULL,
            question_text TEXT
        )`, tableName))
	if err != nil {
		return fmt.Errorf("failed to create questions table: %v", err)
	}

	tableName = fmt.Sprintf("answers_%d", int64(math.Abs(float64(update.Message.Chat.ID))))
	_, err = tx.Exec(fmt.Sprintf(`
        CREATE TABLE IF NOT EXISTS %s (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            user_id INTEGER NOT NULL,
            user_msg_id INTEGER NOT NULL,
            admin_msg_id INTEGER NOT NULL,
            answer_text TEXT NOT NULL
        )`, tableName))
	if err != nil {
		return fmt.Errorf("failed to create answers table: %v", err)
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %v", err)
	}

	return nil
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

func (DB *DataBaseSites) GroupByKeyword(text string) int64 {
	var chatID int64
	err := DB.DB.QueryRow("SELECT chat_id FROM chats WHERE keyword = ?", text).Scan(&chatID)
	if err != nil {
		if err == sql.ErrNoRows {
			return -1
		}
		log.Println(err)
	}
	return chatID
}

func (DB *DataBaseSites) WasInGroup(update tgbotapi.Update, group int64) bool {
	userID := update.Message.Chat.ID
	var groups string
	err := DB.DB.QueryRow("SELECT user_all_groups FROM users WHERE user_id = ?", userID).Scan(&groups)
	if err != nil {
		log.Println(err)
	}
	parts := strings.Split(strings.TrimSpace(groups), " ")
	for _, g := range parts {
		val, err := strconv.ParseInt(g, 10, 64)
		if err != nil {
			log.Println(err)
		}
		if val == group {
			return true
		}
	}
	return false
}

func (DB *DataBaseSites) AddInGroup(update tgbotapi.Update, newGroup int64, wasInGroup bool) {
	userID := update.Message.Chat.ID
	_, err := DB.DB.Exec("UPDATE users SET user_group = ? WHERE user_id = ?", newGroup, userID)
	if err != nil {
		log.Println(err)
	}
	if !wasInGroup {
		var groups string
		err = DB.DB.QueryRow("SELECT user_all_groups FROM users WHERE user_id = ?", userID).Scan(&groups)
		if err != nil {
			log.Println(err)
		}
		groups += " " + strconv.FormatInt(newGroup, 10)
		_, err = DB.DB.Exec("UPDATE users SET user_all_groups = ? WHERE user_id = ?", groups, userID)
		if err != nil {
			log.Println(err)
		}
	}
}

func (DB *DataBaseSites) GetGroupByUserID(userID int) int64 {
	var group int64
	err := DB.DB.QueryRow("SELECT user_group FROM users WHERE user_id = ?", userID).Scan(&group)
	if err != nil {
		log.Println(err)
	}
	return group
}

func (DB *DataBaseSites) AddQuestionFromUser(update tgbotapi.Update, reply tgbotapi.Message) {
	//	update - сообщение которое бот получил
	//	reply - сообщение которое бот переслал в чат (копия, ответ на которую мы ждем)
	_, err := DB.DB.Exec("INSERT INTO not_answered_questions(user_id, admin_msg_id, user_msg_id, user_chat_id, user_group) VALUES (?, ?, ?, ?, ?);",
		update.Message.From.ID, reply.MessageID, update.Message.MessageID, update.Message.Chat.ID, DB.GetGroupByUserID(update.Message.From.ID))
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

func (DB *DataBaseSites) DelQuestionFromUser(msg tgbotapi.Message) {
	_, err := DB.DB.Exec("DELETE FROM not_answered_questions WHERE admin_msg_id = ?", msg.MessageID)
	if err != nil {
		log.Println(err)
	}
}

func (DB *DataBaseSites) GetUserMsgIDByAdminID(adminMsgID int) (int, error) {
	var userMsgID int

	err := DB.DB.QueryRow(`
        SELECT user_msg_id 
        FROM not_answered_questions 
        WHERE admin_msg_id = ?`, adminMsgID).Scan(&userMsgID)

	if err != nil {
		if err == sql.ErrNoRows {
			return 0, fmt.Errorf("сообщение бота с ID %d не найдено", adminMsgID)
		}
		return 0, fmt.Errorf("ошибка при поиске user_msg_id: %v", err)
	}

	return userMsgID, nil
}

func (DB *DataBaseSites) GetQuestionsFromUsers(cnt int, filterGroup int64) ([]int, []int, []int, []int64, []string, []int64) {
	rows, err := DB.DB.Query(`
        SELECT q.user_id, q.admin_msg_id, q.user_msg_id, q.user_chat_id,
               COALESCE(u.user_name || ' ' || u.user_surname, 'Аноним') as user_name,
               q.user_group as user_group
        FROM not_answered_questions q
        LEFT JOIN users u ON q.user_id = u.user_id
        WHERE q.user_group = ?
        LIMIT ?`, filterGroup, cnt)
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	var user_chat_ids, user_groups []int64
	var user_msg_ids, admin_msg_ids, user_ids []int
	var user_names []string

	for rows.Next() {
		var user_msg_id, admin_msg_id, user_id int
		var user_chat_id, user_group int64
		var user_name string

		err := rows.Scan(&user_id, &admin_msg_id, &user_msg_id, &user_chat_id, &user_name, &user_group)
		if err != nil {
			log.Println(err)
			continue
		}

		user_msg_ids = append(user_msg_ids, user_msg_id)
		admin_msg_ids = append(admin_msg_ids, admin_msg_id)
		user_chat_ids = append(user_chat_ids, user_chat_id)
		user_ids = append(user_ids, user_id)
		user_names = append(user_names, user_name)
		user_groups = append(user_groups, user_group)
	}

	return user_ids, admin_msg_ids, user_msg_ids, user_chat_ids, user_names, user_groups
}

func (DB *DataBaseSites) SetNewAdminChatId(nmsg tgbotapi.Message, oldid int) {
	nID := nmsg.MessageID
	_, err := DB.DB.Exec("UPDATE not_answered_questions SET admin_msg_id = ? WHERE admin_msg_id = ?", nID, oldid)
	if err != nil {
		log.Println(err)
	}
}

func (DB *DataBaseSites) SaveUserName(userID int, name, surname string) error {
	_, err := DB.DB.Exec(`
        UPDATE users 
        SET user_name = ?, user_surname = ?
        WHERE user_id = ?`,
		name, surname, userID,
	)
	return err
}

func (DB *DataBaseSites) HasName(userID int) bool {
	var name string
	err := DB.DB.QueryRow("SELECT user_name FROM users WHERE user_id = ?", userID).Scan(&name)
	return err == nil && name != ""
}

func (DB *DataBaseSites) GetName(userID int) string {
	var name, surname string
	err := DB.DB.QueryRow("SELECT user_name, user_surname FROM users WHERE user_id = ?", userID).Scan(&name, &surname)
	if err != nil {
		log.Println(err)
	}
	return name + " " + surname
}

func (DB *DataBaseSites) StopGroupLink(group int64) error {
	_, err := DB.DB.Exec("UPDATE chats SET invitable = ? WHERE chat_id = ?", 0, group)
	return err
}

func (DB *DataBaseSites) ContinueGroupLink(group int64) error {
	_, err := DB.DB.Exec("UPDATE chats SET invitable = ? WHERE chat_id = ?", 1, group)
	return err
}

func (DB *DataBaseSites) IsInvitable(group int64) bool {
	var invitable int
	err := DB.DB.QueryRow("SELECT invitable FROM chats WHERE chat_id = ?", group).Scan(&invitable)
	fmt.Println(invitable, group)
	return err == nil && (invitable == 1)
}

func (DB *DataBaseSites) BanUser(userID int, group int64) error {
	_, err := DB.DB.Exec("INSERT INTO banned(user_id, user_group) VALUES (?, ?)", userID, group)
	return err
}

func (DB *DataBaseSites) UnBanUser(userID int, group int64) error {
	_, err := DB.DB.Exec("DELETE FROM banned WHERE user_id = ? AND user_group = ?", userID, group)
	return err
}

func (DB *DataBaseSites) IsBanned(userID int, group int64) bool {
	var exists bool
	err := DB.DB.QueryRow("SELECT EXISTS(SELECT 1 FROM banned WHERE user_id = ? AND user_group = ?)", userID, group).Scan(&exists)
	if err != nil {
		log.Println(err)
	}
	return exists
}

func (DB *DataBaseSites) GetUserIDByMsgIDInAdminChat(msgID int) (int, error) {
	var userID int
	err := DB.DB.QueryRow("SELECT user_id FROM not_answered_questions WHERE admin_msg_id = ?", msgID).Scan(&userID)
	return userID, err
}

func (DB *DataBaseSites) DeleteQuestionsFromUser(userID int) int64 {
	result, err := DB.DB.Exec("DELETE FROM not_answered_questions WHERE user_id = ?", userID)
	if err != nil {
		log.Println(err)
	}
	ret, err := result.RowsAffected()
	if err != nil {
		log.Println(err)
	}
	return ret
}

func (DB *DataBaseSites) DeleteQuestionsByUsers(db string, group int64) (int, error) {
	tx, err := DB.DB.Begin()
	if err != nil {
		return 0, err
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	rows, err := tx.Query(fmt.Sprintf("SELECT user_id FROM %s WHERE user_group = %d", db, group))
	if err != nil {
		return 0, fmt.Errorf("query failed: %v", err)
	}
	defer rows.Close()

	cnt := 0
	for rows.Next() {
		var userID int
		if err := rows.Scan(&userID); err != nil {
			log.Printf("scan error: %v", err)
			continue
		}

		res, err := tx.Exec("DELETE FROM not_answered_questions WHERE user_id = ? AND user_group = ?", userID, group)
		if err != nil {
			log.Println("delete error for user %d: %v", userID, err)
			continue
		}
		affected, _ := res.RowsAffected()
		cnt += int(affected)
	}

	if err = rows.Err(); err != nil {
		return cnt, err
	}

	if err = tx.Commit(); err != nil {
		return cnt, err
	}

	return cnt, nil
}

func (DB *DataBaseSites) DeleteQuestionsFromBannedUsers(group int64) (int, error) {
	return DB.DeleteQuestionsByUsers("banned", group)
}

func (DB *DataBaseSites) DeleteQuestionsFromUsers(group int64) (int, error) {
	return DB.DeleteQuestionsByUsers("users", group)
}

func (DB *DataBaseSites) AddQuestionFromAdmin(update tgbotapi.Update) (int64, error) {
	text := update.Message.CommandArguments()

	chatID := int64(math.Abs(float64(update.Message.Chat.ID)))
	adminMsgID := update.Message.MessageID
	tableName := fmt.Sprintf("questions_%d", chatID)

	result, err := DB.DB.Exec(fmt.Sprintf(`
        INSERT INTO %s (admin_msg_id, question_text)
        VALUES (?, ?)`, tableName),
		adminMsgID, text)
	if err != nil {
		return 0, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("failed to get last insert ID: %v", err)
	}

	return id, nil
}

func (DB *DataBaseSites) GetFirstNotAnsweredQuestion(userID int) (adminMsgID int, groupID int64, err error) {
	groupID = DB.GetGroupByUserID(userID)
	if groupID == 0 {
		return 0, 0, fmt.Errorf("user has no group assigned")
	}

	groupABS := int64(math.Abs(float64(groupID)))
	questionsTable := fmt.Sprintf("questions_%d", groupABS)
	answersTable := fmt.Sprintf("answers_%d", groupABS)

	query := fmt.Sprintf(`
        SELECT q.admin_msg_id 
        FROM %s q
        WHERE NOT EXISTS (
            SELECT 1 FROM %s a 
            WHERE a.admin_msg_id = q.admin_msg_id 
            AND a.user_id = ?
        )
        ORDER BY q.id ASC
        LIMIT 1`, questionsTable, answersTable)

	err = DB.DB.QueryRow(query, userID).Scan(&adminMsgID)
	if err != nil {
		if err == sql.ErrNoRows {
			return 0, groupID, fmt.Errorf("no unanswered questions")
		}
		return 0, groupID, fmt.Errorf("database error: %v", err)
	}

	return adminMsgID, groupID, nil
}

func (DB *DataBaseSites) IsAdminQuestion(adminMsgID int, groupID int64) (bool, error) {
	tableName := fmt.Sprintf("questions_%d", int64(math.Abs(float64(groupID))))

	var exists bool
	err := DB.DB.QueryRow(fmt.Sprintf(`
        SELECT EXISTS(
            SELECT 1 FROM %s 
            WHERE admin_msg_id = ?
        )`, tableName), adminMsgID).Scan(&exists)

	if err != nil {
		return false, fmt.Errorf("failed to check question existence: %v", err)
	}

	return exists, nil
}

func (DB *DataBaseSites) DidUserAnswered(userID, adminMsgID int, groupID int64) (bool, error) {
	tableName := fmt.Sprintf("answers_%d", int64(math.Abs(float64(groupID))))

	var exists bool
	err := DB.DB.QueryRow(fmt.Sprintf(`
        SELECT EXISTS(
            SELECT 1 FROM %s 
            WHERE user_id = ? AND admin_msg_id = ?
        )`, tableName), userID, adminMsgID).Scan(&exists)

	if err != nil {
		return false, fmt.Errorf("failed to check answer existence: %v", err)
	}

	return exists, nil
}

func (DB *DataBaseSites) AddUserAnswerOnAdminQuestion(userID, userMsgID, adminMsgID int, groupID int64, answer string) error {
	tableName := fmt.Sprintf("answers_%d", int64(math.Abs(float64(groupID))))

	_, err := DB.DB.Exec(fmt.Sprintf(`
        INSERT INTO %s (user_id, user_msg_id, admin_msg_id, answer_text)
        VALUES (?, ?, ?, ?)`, tableName),
		userID, userMsgID, adminMsgID, answer)

	if err != nil {
		return fmt.Errorf("failed to insert answer: %v", err)
	}

	return nil
}

func (DB *DataBaseSites) GetAdminQuestionID(adminMsgID int, groupID int64) (int, error) {
	tableName := fmt.Sprintf("questions_%d", int64(math.Abs(float64(groupID))))

	var questionID int
	err := DB.DB.QueryRow(fmt.Sprintf(`
        SELECT id FROM %s 
        WHERE admin_msg_id = ?`, tableName), adminMsgID).Scan(&questionID)

	if err != nil {
		if err == sql.ErrNoRows {
			return 0, fmt.Errorf("question not found")
		}
		return 0, fmt.Errorf("database error: %v", err)
	}

	return questionID, nil
}

func (DB *DataBaseSites) DelAdminQuestion(adminMsgID int, groupID int64) error {
	tx, err := DB.DB.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %v", err)
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	questionsTable := fmt.Sprintf("questions_%d", int64(math.Abs(float64(groupID))))
	answersTable := fmt.Sprintf("answers_%d", int64(math.Abs(float64(groupID))))

	_, err = tx.Exec(fmt.Sprintf(`
        DELETE FROM %s 
        WHERE admin_msg_id = ?`, answersTable), adminMsgID)
	if err != nil {
		return fmt.Errorf("failed to delete from answers: %v", err)
	}

	_, err = tx.Exec(fmt.Sprintf(`
        DELETE FROM %s 
        WHERE admin_msg_id = ?`, questionsTable), adminMsgID)
	if err != nil {
		return fmt.Errorf("failed to delete from questions: %v", err)
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %v", err)
	}

	return nil
}

func (DB *DataBaseSites) DelLastAdminQuestion(groupID int64) error {
	questionsTable := fmt.Sprintf("questions_%d", int64(math.Abs(float64(groupID))))

	var lastAdminMsgID int
	err := DB.DB.QueryRow(fmt.Sprintf(`
        SELECT admin_msg_id FROM %s 
        ORDER BY id DESC 
        LIMIT 1`, questionsTable)).Scan(&lastAdminMsgID)

	if err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("no questions found in group")
		}
		return fmt.Errorf("failed to get last question: %v", err)
	}

	return DB.DelAdminQuestion(lastAdminMsgID, groupID)
}

func (DB *DataBaseSites) GetAnswersForQuestion(adminMsgID int, groupID int64) (map[int]string, error) {
	answersTable := fmt.Sprintf("answers_%d", int64(math.Abs(float64(groupID))))

	// Запрос всех ответов на вопрос
	rows, err := DB.DB.Query(fmt.Sprintf(`
        SELECT user_id, answer_text 
        FROM %s 
        WHERE admin_msg_id = ? 
        ORDER BY id ASC`, answersTable), adminMsgID)
	if err != nil {
		return nil, fmt.Errorf("failed to query answers: %v", err)
	}
	defer rows.Close()

	results := make(map[int]string)

	var userID int
	var answerText string

	for rows.Next() {
		if err := rows.Scan(&userID, &answerText); err != nil {
			return nil, fmt.Errorf("failed to scan answer: %v", err)
		}
		results[userID] = answerText
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %v", err)
	}

	return results, nil
}

func (DB *DataBaseSites) GetAdminMsgIDByQuestionIDAndGroupID(questionID int, groupID int64) (int, error) {
	tableName := fmt.Sprintf("questions_%d", int64(math.Abs(float64(groupID))))

	var adminMsgID int
	err := DB.DB.QueryRow(fmt.Sprintf(`
        SELECT admin_msg_id FROM %s 
        WHERE id = ?`, tableName), questionID).Scan(&adminMsgID)

	if err != nil {
		if err == sql.ErrNoRows {
			return 0, fmt.Errorf("question with ID %d not found in group %d", questionID, groupID)
		}
		return 0, fmt.Errorf("database error: %v", err)
	}

	return adminMsgID, nil
}

func (DB *DataBaseSites) GetUsernameByUserID(userID int) (string, error) {
	var username string

	err := DB.DB.QueryRow("SELECT username FROM users WHERE user_id = ?", userID).Scan(&username)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", fmt.Errorf("user with ID %d not found", userID)
		}
		return "", fmt.Errorf("error querying database: %v", err)
	}

	return username, nil
}

func (DB *DataBaseSites) GetUserIDByUsername(username string) (int, error) {
	var userID int

	err := DB.DB.QueryRow("SELECT user_id FROM users WHERE username = ?", username).Scan(&userID)
	if err != nil {
		if err == sql.ErrNoRows {
			return 0, fmt.Errorf("user with username %s not found", username)
		}
		return 0, fmt.Errorf("error querying database: %v", err)
	}

	return userID, nil
}

func (DB *DataBaseSites) SetUsername(userID int, userName string) error {
	_, err := DB.DB.Exec("UPDATE users SET username = ? WHERE user_id = ?", userName, userID)
	return err
}
