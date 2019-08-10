package main

import (
	"github.com/bwmarrin/discordgo"
	_ "github.com/joho/godotenv/autoload"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
)

var dg *discordgo.Session

func main() {
	// create bot client
	var err error
	dg, err = discordgo.New("Bot " + os.Getenv("DISCORD_BOT_TOKEN"))
	if err != nil {
		log.Fatalf("err creating discord session: %v", err)
	}

	// register handlers
	dg.AddHandler(messageCreate)

	// open connection
	err = dg.Open()
	if err != nil {
		log.Fatalf("err opening connection: %v", err)
	}

	// listen for interrupts to disconnect discord correctly
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
	<-sc
	err = dg.Close()
	if err != nil {
		log.Fatalf("err closing connection: %v", err)
	}
}

func messageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	// ignore bots own messages
	if m.Author.ID == s.State.User.ID {
		return
	}

	// ignore empty messages (e.g. images)
	if m.Content == "" {
		return
	}

	// get the last message
	messages, err := dg.ChannelMessages(m.ChannelID, 3, m.ID, "", "")
	if err != nil {
		log.Printf("err getting last message: %v", err)
		return
	}

	trimmedCurrentContent := strings.TrimSpace(m.Content)
	_, err = strconv.Atoi(trimmedCurrentContent)
	currentIsNumber := err == nil

	// count issues
	issues := 0
	for _, message := range messages {
		trimmedContent := strings.TrimSpace(message.Content)
		_, err = strconv.Atoi(trimmedContent)
		isNumber := err == nil

		if (trimmedCurrentContent == trimmedContent) || (currentIsNumber && isNumber) {
			issues++
		}
	}

	err = nil
	// perform action depending on repeat count
	if issues == 1 {
		err = dg.MessageReactionAdd(m.ChannelID, m.ID, "😞") // disappointed
	} else if issues == 2 {
		err = dg.MessageReactionAdd(m.ChannelID, m.ID, "😡") // angry
	} else if issues >= 3 {
		err = dg.ChannelMessageDelete(m.ChannelID, m.ID)
	}
	if err != nil {
		log.Printf("err performing message action: %v", err)
	}
}