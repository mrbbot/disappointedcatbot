package main

import (
	"encoding/json"
	"fmt"
	"github.com/bwmarrin/discordgo"
	_ "github.com/joho/godotenv/autoload"
	"github.com/writeas/go-strip-markdown"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"
)

type Command struct {
	Command    string `json:"command"`
	Response   string `json:"response"`
	IsRegexp   bool   `json:"regexp,omitempty"`
	Contains   bool   `json:"contains,omitempty"`
	ShowTyping bool   `json:"typing,omitempty"`

	regexp *regexp.Regexp
}

func (r *Command) Regexp() *regexp.Regexp {
	if r.regexp == nil {
		r.regexp = regexp.MustCompile(r.Command)
	}
	return r.regexp
}

func (r *Command) String() string {
	return fmt.Sprintf(
		"(%s -> %s (is regexp: %t, contains: %t, show typing: %t))",
		r.Command, r.Response, r.IsRegexp, r.Contains, r.ShowTyping,
	)
}

type Config struct {
	Commands         []*Command      `json:"commands,omitempty"`
	ExcludedChannels map[string]bool `json:"excluded_channels,omitempty"`
	ExcludedUsers    map[string]bool `json:"excluded_users,omitempty"`
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

	// ignore excluded channels/users
	if v, _ := config.ExcludedChannels[m.ChannelID]; v {
		return
	}
	if v, _ := config.ExcludedUsers[m.Author.ID]; v {
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

	// check & send command responses
	for _, command := range config.Commands {
		var response string
		if command.IsRegexp {
			submatch := command.Regexp().FindStringSubmatch(m.Content)
			if len(submatch) > 0 {
				response = command.Response
				for i, submatchPart := range submatch {
					response = strings.ReplaceAll(response, "$"+strconv.Itoa(i), submatchPart)
				}
			}
		} else if (command.Contains && strings.Contains(trimmedCurrentContent, command.Command)) || (!command.Contains && trimmedCurrentContent == command.Command) {
			response = command.Response
		}
		if response != "" {
			if command.ShowTyping {
				err = dg.ChannelTyping(m.ChannelID)
				if err != nil {
					log.Printf("err sending typing indicator: %v", err)
				} else {
					go func() {
						time.Sleep(time.Second)
						_, err = dg.ChannelMessageSend(m.ChannelID, response)
						if err != nil {
							log.Printf("err sending message: %v", err)
						}
					}()
				}
			} else {
				_, err = dg.ChannelMessageSend(m.ChannelID, response)
				if err != nil {
					log.Printf("err sending message: %v", err)
				}
			}
		}
	}
}
