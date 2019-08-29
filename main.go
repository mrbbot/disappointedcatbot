package main

import (
	"encoding/json"
	"github.com/bwmarrin/discordgo"
	_ "github.com/joho/godotenv/autoload"
	"github.com/writeas/go-strip-markdown"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"regexp"
	"strings"
	"syscall"
)

type ReplacementResponse struct {
	From     string `json:"from"`
	To       string `json:"to"`
	IsRegexp bool   `json:"regexp,omitempty"`

	regexp *regexp.Regexp
}

func (r *ReplacementResponse) Regexp() *regexp.Regexp {
	if r.regexp == nil {
		r.regexp = regexp.MustCompile(r.From)
	}
	return r.regexp
}

type Config struct {
	Replacements []*ReplacementResponse `json:"replacements,omitempty"`
	Responses    []*ReplacementResponse `json:"responses,omitempty"`
}

var (
	dg     *discordgo.Session
	config *Config
)

func init() {
	data, err := ioutil.ReadFile("config.json")
	if err != nil && !os.IsNotExist(err) {
		log.Fatalf("err reading config file: %v", err)
	} else if err == nil {
		config = &Config{}
		err = json.Unmarshal(data, config)
		if err != nil {
			log.Fatalf("err parsing config file: %v", err)
		}
	}
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

	// count issues
	issues := 0
	for _, message := range messages {
		trimmedContent := stripMessage(message)

		if trimmedCurrentContent == trimmedContent {
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

	// make replacements
	originalContent := m.Content
	newContent := m.Content
	for _, replacement := range config.Replacements {
		if replacement.IsRegexp {
			newContent = replacement.Regexp().ReplaceAllString(newContent, replacement.To)
		} else {
			newContent = strings.ReplaceAll(newContent, replacement.From, replacement.To)
		}
	}
	if newContent != originalContent {
		_, err = dg.ChannelMessageEdit(m.ChannelID, m.ID, newContent)
		if err != nil {
			log.Printf("err editing message: %v", err)
		}
	}

	// send responses
	for _, response := range config.Responses {
		var contains bool
		if response.IsRegexp {
			contains = response.Regexp().MatchString(originalContent)
		} else {
			contains = strings.Contains(originalContent, response.From)
		}
		if contains {
			_, err = dg.ChannelMessageSend(m.ChannelID, response.To)
			if err != nil {
				log.Printf("err sending message: %v", err)
			}
		}
	}
}
