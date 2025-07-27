package password

import (
	"database/sql"
	"log"
	"math/rand"
	"strings"
	"time"
)

func quickRandomDigits(length int) string {
	rand.Seed(time.Now().UnixNano())
	var sb strings.Builder
	sb.Grow(length)
	for i := 0; i < length; i++ {
		sb.WriteByte(byte(rand.Intn(10) + '0'))
	}
	return sb.String()
}

func GetPassword(DB *sql.DB, len int) string {
	password := quickRandomDigits(len)
	_, err := DB.Exec("INSERT INTO passwords(password) VALUES (?);", password)
	if err != nil {
		log.Fatal(err)
	}
	return password
}
