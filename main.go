package main

import (
	"github.com/bwmarrin/discordgo"
	_ "github.com/joho/godotenv/autoload"
	"github.com/writeas/go-strip-markdown"
	"log"
	"os"
	"os/signal"
	"regexp"
	"strings"
	"syscall"
)

var (
	dg      *discordgo.Session
	numbers = []string{
		"0", "1", "2", "3", "4", "5", "6", "7", "8", "9",
		"0⃣ ", "1⃣", "2⃣", "3⃣", "4⃣", "5⃣", "6⃣", "7⃣", "8⃣", "9⃣",
		"0️⃣", "1️⃣", "2️⃣", "3️⃣", "4️⃣", "5️⃣", "6️⃣", "7️⃣", "8️⃣", "9️⃣", "🔟",
		"zero", "one", "two", "three", "four", "five", "six", "seven", "eight", "nine", "ten",
		"eleven", "twelve", "thirteen", "fifteen",
		"twenty", "thirty", "forty", "fifty", "sixty", "seventy", "eighty", "ninety",
		"hundred", "thousand", "million", "billion", "trillion",
	}
	numberRegex *regexp.Regexp
)

func init() {
	numberRegexString := "("
	for i, number := range numbers {
		numberRegexString += number
		if i < len(numbers)-1 {
			numberRegexString += "|"
		}
	}
	numberRegexString += ")"
	numberRegex = regexp.MustCompile(numberRegexString)
}

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

func stripMessage(m *discordgo.Message) string {
	content := m.Content
	content = strings.ToLower(content)
	content = stripmd.Strip(content)
	content = strings.TrimSpace(content)
	return content
}

func isNumber(s string) bool {
	return numberRegex.MatchString(s)
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

	trimmedCurrentContent := stripMessage(m.Message)
	currentIsNumber := isNumber(trimmedCurrentContent)

	// count issues
	issues := 0
	for _, message := range messages {
		trimmedContent := stripMessage(message)
		isNumber := isNumber(trimmedContent)

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
