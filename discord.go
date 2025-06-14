package main

import (
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"streambot/backoff"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/fatih/color"
)

var discordColor = color.New(color.FgMagenta)
var discordBotToken string  // Discord bot token from secrets
var discordChannelID string // Discord channel ID to monitor

const DISCORD_ICON = `<img src="discord.svg" class="emoji">`

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

		// Set the intents to receive message content and other necessary permissions
		dg.Identify.Intents |= discordgo.IntentsGuildMessages
		dg.Identify.Intents |= discordgo.IntentsMessageContent
		dg.Identify.Intents |= discordgo.IntentsGuilds

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
		discordColor.Printf("Monitoring Discord channel ID: %s\n", discordChannelID)

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
	// Add recovery for any panics
	defer func() {
		if r := recover(); r != nil {
			discordColor.Printf("Recovered from panic in Discord message handler: %v\n", r)
		}
	}()

	// Ignore messages from the bot itself
	if m.Author.ID == s.State.User.ID {
		return
	}

	// Only process messages from the configured channel
	if m.ChannelID != discordChannelID {
		return
	}

	// Convert Discord message to the standard ChatEntry format
	username := m.Author.GlobalName
	if username == "" {
		username = m.Author.Username
	}
	discordUser := DiscordUser{
		ID:       m.Author.ID,
		Username: username,
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
	attachmentHTML := ""
	attachmentText := ""

	// Handle attachments if any
	if len(m.Attachments) > 0 {
		for _, attachment := range m.Attachments {

			filename, err := downloadDiscordAttachment(attachment, m.ID)
			if err != nil {
				discordColor.Printf("Failed to download attachment %s: %v\n", attachment.Filename, err)
				continue
			}
			attachmentText += fmt.Sprintf("[attachment %s]", attachment.Filename)
			attachmentHTML += fmt.Sprintf("<a href=\"attachments/%s\">", filename)
			// If it's an image, download it and add to HTML
			if isImageFile(attachment.Filename) {
				if err != nil {
					discordColor.Printf("Failed to download attachment %s: %v\n", attachment.Filename, err)
				} else {
					// Convert to web path (replace backslashes with forward slashes for web)
					attachmentHTML += fmt.Sprintf(`<img src="attachments/%s" class="attachment">`, filename)
				}
			} else {
				attachmentHTML += attachment.Filename
			}
			attachmentHTML += "</a>"
		}
	}

	// Handle embeds (for GIFs from Tenor, Giphy, etc.)
	if len(m.Embeds) > 0 {
		content = ""
		for _, embed := range m.Embeds {
			attachmentText += fmt.Sprintf("[embed %s]", embed.URL)
			// Check if the embed has an image (like GIFs)
			if embed.Image != nil && embed.Image.URL != "" {
				// Download the embed image (GIF)
				filename, err := downloadDiscordEmbedImage(embed.Image, m.ID)
				if err != nil {
					discordColor.Printf("Failed to download embed image %s: %v\n", embed.Image.URL, err)
					// If download fails, just show the image directly from the URL
					attachmentHTML += fmt.Sprintf(`<img src="%s" class="attachment">`, embed.Image.URL)
				} else {
					// Use the downloaded image
					attachmentHTML += fmt.Sprintf(`<img src="attachments/%s" class="attachment">`, filename)
				}
			}
			// Check if the embed has a video (less common but possible)
			if embed.Video != nil && embed.Video.URL != "" {
				attachmentHTML += fmt.Sprintf(`<video src="%s" class="attachment" autoplay loop controls></video>`, embed.Video.URL)
			}
		}
	}

	// Handle mentions and replace them with proper names
	for _, mention := range m.Mentions {
		mentionText := fmt.Sprintf("<@%s>", mention.ID)
		replacementText := fmt.Sprintf("@%s", mention.Username)
		content = strings.Replace(content, mentionText, replacementText, -1)
		textOnly = strings.Replace(textOnly, mentionText, replacementText, -1)
	}

	// Create chat entry
	chatEntry := ChatEntry{
		Author:           *user,
		OriginalMessage:  content,
		DiscordMessageID: m.ID,
		timestamp:        time.Now(),
		textOnly:         textOnly,
		ttsMsg:           VocalizeHTML(content),
		terminalMsg:      fmt.Sprintf("%s: %s%s\n", user.DisplayName(), content, attachmentText),
		HTML:             fmt.Sprintf(DISCORD_ICON+` %s: %s%s`, user.HTML(), html.EscapeString(content), attachmentHTML),
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

// SendDiscordMessage sends a message to the configured Discord channel
func SendDiscordMessage(message string) error {
	if discordSession == nil {
		return fmt.Errorf("Discord session not initialized")
	}
	_, err := discordSession.ChannelMessageSend(discordChannelID, message)
	return err
}

var discordSession *discordgo.Session

// isImageFile checks if a filename represents an image file
func isImageFile(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	imageExts := []string{".jpg", ".jpeg", ".png", ".gif", ".webp", ".bmp", ".svg"}
	for _, imgExt := range imageExts {
		if ext == imgExt {
			return true
		}
	}
	return false
}

// downloadDiscordAttachment downloads a Discord attachment to the static/attachments directory.
// Returns filename and error.
func downloadDiscordAttachment(attachment *discordgo.MessageAttachment, discordMessageID string) (string, error) {
	// Create filename with the format {discord_message_id}_{attachment_name}
	filename := fmt.Sprintf("%s_%s", discordMessageID, attachment.Filename)
	localPath := filepath.Join("static", "attachments", filename)

	// Download the file
	resp, err := http.Get(attachment.URL)
	if err != nil {
		return "", fmt.Errorf("failed to download attachment: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to download attachment, status: %d", resp.StatusCode)
	}

	// Create the file
	file, err := os.Create(localPath)
	if err != nil {
		return "", fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	// Copy the content
	_, err = io.Copy(file, resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to save attachment: %w", err)
	}

	return filename, nil
}

// downloadDiscordEmbedImage downloads a Discord embed image (like GIF) to the static/attachments directory.
// Returns filename and error.
func downloadDiscordEmbedImage(embedImage *discordgo.MessageEmbedImage, discordMessageID string) (string, error) {
	// Extract filename from URL or create one
	parsedURL, err := url.Parse(embedImage.URL)
	if err != nil {
		return "", fmt.Errorf("failed to parse embed image URL: %w", err)
	}

	// Get the file extension from the URL path
	ext := filepath.Ext(parsedURL.Path)
	if ext == "" {
		// Default to .gif for Tenor/Giphy URLs
		ext = ".gif"
	}

	// Create filename with the format {discord_message_id}_embed_{timestamp}{extension}
	timestamp := time.Now().Unix()
	filename := fmt.Sprintf("%s_embed_%d%s", discordMessageID, timestamp, ext)
	localPath := filepath.Join("static", "attachments", filename)

	// Download the file
	resp, err := http.Get(embedImage.URL)
	if err != nil {
		return "", fmt.Errorf("failed to download embed image: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to download embed image, status: %d", resp.StatusCode)
	}

	// Create the file
	file, err := os.Create(localPath)
	if err != nil {
		return "", fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	// Copy the content
	_, err = io.Copy(file, resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to save embed image: %w", err)
	}

	return filename, nil
}

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
