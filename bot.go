package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"

	_ "github.com/lib/pq"
	"gopkg.in/telegram-bot-api.v4"
)

var (
	DatabaseUrl string
	BotToken    string
	WebHookUrl  string
	Port        string
)

// Bot message type
const (
	Group      = "group"
	SuperGroup = "supergroup"
	Private    = "private"
)

var (
	bot *tgbotapi.BotAPI
	db  *sql.DB
)

func GetEnvVars() {
	DatabaseUrl = os.Getenv("DATABASE_URL")
	BotToken = os.Getenv("BOT_TOKEN")
	WebHookUrl = os.Getenv("WEBHOOK_URL")
	Port = ":" + os.Getenv("PORT")
}

func ProcessUpdates(update *tgbotapi.Update) {
	switch update.Message.Chat.Type {
	case Group, SuperGroup:
		if update.Message.GroupChatCreated ||
			update.Message.ChannelChatCreated ||
			update.Message.SuperGroupChatCreated {
			RegisterChat(update.Message.Chat.ID)
		}

		if update.Message.NewChatMembers != nil {
			for i := 0; i < len(*update.Message.NewChatMembers); i++ {
				if (*update.Message.NewChatMembers)[i].FirstName == "MessageBroadcaster" {
					RegisterChat(update.Message.Chat.ID)
				}
			}
		}

		if update.Message.LeftChatMember != nil &&
			update.Message.LeftChatMember.FirstName == "MessageBroadcaster" {
			UnregisterChat(update.Message.Chat.ID)
		}
	case Private:
		BroadcastMessage(update.Message.Text)
	default:
		NotImplemented(update.Message.Chat.ID)
	}
}

func RegisterChat(chatID int64) {
	if _, err := db.Exec("INSERT INTO ChatIDs VALUES ($1)", chatID); err != nil {
		log.Fatalf("Error inserting into table: %q", err)
	}
}

func UnregisterChat(chatID int64) {
	if _, err := db.Exec("DELETE FROM ChatIDs WHERE id = $1", chatID); err != nil {
		log.Fatalf("Error deleting from table: %q", err)
	}
}

func BroadcastMessage(message string) {
	rows, err := db.Query("SELECT id FROM ChatIDs")
	if err != nil {
		log.Fatalf("Error selecting from table: %q", err)
	}

	for rows.Next() {
		var chatID int32
		rows.Scan(&chatID)

		bot.Send(tgbotapi.NewMessage(
			int64(chatID),
			message,
		))
	}
}

func NotImplemented(chatID int64) {
	bot.Send(tgbotapi.NewMessage(
		chatID,
		"Unknown chat type. Pls contact with developer.",
	))
}

func SetUpDatabase() {
	var err error
	if db, err = sql.Open("postgres", DatabaseUrl); err != nil {
		log.Fatalf("Error opening database: %q", err)
	}

	if _, err = db.Exec("CREATE TABLE IF NOT EXISTS ChatIds (id BIGSERIAL PRIMARY KEY)"); err != nil {
		log.Fatalf("Error creating table: %q", err)
	}

	var rows *sql.Rows
	if rows, err = db.Query("SELECT id FROM ChatIDs"); err != nil {
		log.Fatalf("Error selecting from table: %q", err)
	}

	for rows.Next() {
		var chatID int32
		rows.Scan(&chatID)

		if _, err = bot.GetChat(tgbotapi.ChatConfig{ChatID: int64(chatID)}); err != nil {
			if _, err = db.Exec("DELETE FROM ChatIDs WHERE id = $1", chatID); err != nil {
				log.Fatalf("Error deleting from table: %q", err)
			}
		}
	}
}

func main() {
	var err error

	GetEnvVars()

	if bot, err = tgbotapi.NewBotAPI(BotToken); err != nil {
		log.Fatalf("Error connecting to API: %q", err)
	}

	SetUpDatabase()

	if _, err = bot.SetWebhook(tgbotapi.NewWebhook(WebHookUrl)); err != nil {
		log.Fatalf("Error setting webhook: %q", err)
	}

	go http.ListenAndServe(Port, nil)
	fmt.Println("server started on port " + Port)

	updates := bot.ListenForWebhook("/")

	for update := range updates {
		ProcessUpdates(&update)
	}
}
