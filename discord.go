package main

import (
	"fmt"
	"html"
	"streambot/backoff"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/fatih/color"
)

var discordColor = color.New(color.FgMagenta)
var discordBotToken string  // Discord bot token from secrets
var discordChannelID string // Discord channel ID to monitor

type DiscordUser struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Avatar   string `json:"avatar,omitempty"`
}

func (u DiscordUser) DisplayName() string {
	return u.Username
}

func (u DiscordUser) Key() string {
	return DISCORD_KEY_PREFIX + u.ID
}

// Initialize the Discord session, connect to the server, and start listening for messages
func DiscordChatBot() {
	backoff := backoff.Backoff{
		Color:       discordColor,
		Description: "Discord Chat",
	}

	for {
		backoff.Attempt()
		Webserver.Call("Ping", "Discord")

		// Create a new Discord session
		dg, err := discordgo.New("Bot " + discordBotToken)
		if err != nil {
			discordColor.Println("Error creating Discord session:", err)
			continue
		}

		// Set the intent to receive message content
		dg.Identify.Intents = discordgo.IntentsGuildMessages

		// Register handler for messages
		dg.AddHandler(messageHandler)

		// Open a websocket connection to Discord
		err = dg.Open()
		if err != nil {
			discordColor.Println("Error opening Discord connection:", err)
			dg.Close()
			continue
		}

		discordColor.Println("Discord bot is now running.")

		// Store the session globally so it can be used for message deletion
		discordSession = dg

		// Keep connection alive until it fails
		errorChan := make(chan error)
		heartbeatTicker := time.NewTicker(30 * time.Second)
		defer heartbeatTicker.Stop()

		// Set up a heartbeat check
		go func() {
			for {
				// Wait for the heartbeat interval
				<-heartbeatTicker.C

				// Try to get the gateway latency - this will fail if disconnected
				_, err := dg.GatewayBot()
				if err != nil {
					errorChan <- fmt.Errorf("discord gateway check failed: %w", err)
					return
				}
			}
		}()

		// Wait for an error or external close
		select {
		case err := <-errorChan:
			discordColor.Println("Discord connection error:", err)
			dg.Close()
			break
		}
	}
}

// Handle incoming Discord messages
func messageHandler(s *discordgo.Session, m *discordgo.MessageCreate) {
	// Ignore messages from the bot itself
	if m.Author.ID == s.State.User.ID {
		return
	}

	// Only process messages from the configured channel
	if m.ChannelID != discordChannelID {
		return
	}

	// Convert Discord message to the standard ChatEntry format
	discordUser := DiscordUser{
		ID:       m.Author.ID,
		Username: m.Author.Username,
		Avatar:   m.Author.Avatar,
	}

	user, exists := DiscordIndex[discordUser.Key()]
	if !exists {
		user = &User{
			DiscordUser: &discordUser,
		}
		DiscordIndex[discordUser.Key()] = user
	}

	content := m.Content
	textOnly := content

	// Create chat entry
	chatEntry := ChatEntry{
		Author:           *user,
		OriginalMessage:  content,
		DiscordMessageID: m.ID,
		timestamp:        time.Now(),
		textOnly:         textOnly,
		ttsMsg:           VocalizeHTML(content),
		terminalMsg:      fmt.Sprintf("%s: %s\n", user.DisplayName(), content),
		HTML:             fmt.Sprintf(`<div class="message">%s: %s</div>`, user.HTML(), html.EscapeString(content)),
	}

	// Process message in the main channel
	MainChannel <- func() {
		MainOnChatEntry(chatEntry)
	}

	discordColor.Printf("Discord message from %s: %s\n", user.DisplayName(), content)
}

// Delete a Discord message
func DeleteDiscordMessage(channelID, messageID string) error {
	if discordSession == nil {
		return fmt.Errorf("Discord session not initialized")
	}
	return discordSession.ChannelMessageDelete(channelID, messageID)
}

var discordSession *discordgo.Session

// LoadDiscordAuth loads Discord authentication information from secrets file
func LoadDiscordAuth() error {
	var err error
	discordBotToken, err = ReadStringFromFile(secretsPath("discord_token.txt"))
	if err != nil {
		return fmt.Errorf("couldn't read Discord token: %w", err)
	}

	discordChannelID, err = ReadStringFromFile(secretsPath("discord_channel.txt"))
	if err != nil {
		return fmt.Errorf("couldn't read Discord channel ID: %w", err)
	}

	return nil
}

// Initialize Discord bot connection
func InitDiscord() {
	err := LoadDiscordAuth()
	if err != nil {
		discordColor.Println("Error loading Discord auth:", err)
		return
	}

	go DiscordChatBot()
}
