package main

import (
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
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

// URL patterns for GIF services
var tenorURLPattern = regexp.MustCompile(`(?i)https?://tenor\.com/view/[^/\s]+-(\d+)`)
var giphyURLPattern = regexp.MustCompile(`(?i)https?://giphy\.com/gifs/[^/\s]*-([a-zA-Z0-9]+)`)
var giphyMediaPattern = regexp.MustCompile(`(?i)https?://media\.giphy\.com/media/([a-zA-Z0-9]+)/.*\.gif`)

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

// containsGIFServiceURL reports whether content links to a GIF service that
// Discord is expected to unfurl into an embed.
func containsGIFServiceURL(content string) bool {
	return tenorURLPattern.MatchString(content) ||
		giphyURLPattern.MatchString(content) ||
		giphyMediaPattern.MatchString(content)
}

// waitForMessageEmbeds re-fetches a message a few times, waiting for Discord
// to unfurl its links into embeds. MESSAGE_CREATE usually arrives before the
// unfurling completes, so the embeds only show up on a later fetch. Returns
// nil if no embeds appeared within the polling window.
func waitForMessageEmbeds(s *discordgo.Session, channelID, messageID string) []*discordgo.MessageEmbed {
	for attempt := 0; attempt < 4; attempt++ {
		time.Sleep(800 * time.Millisecond)
		msg, err := s.ChannelMessage(channelID, messageID)
		if err != nil {
			discordColor.Printf("Failed to re-fetch message %s while waiting for embeds: %v\n", messageID, err)
			return nil
		}
		if len(msg.Embeds) > 0 {
			return msg.Embeds
		}
	}
	return nil
}

// detectAndProcessGIFURLs is a fallback for GIF links that Discord didn't
// unfurl into an embed. Giphy URLs map to predictable media URLs so they can
// be rendered without an API; Tenor URLs can't (the Tenor API shuts down on
// 2026-06-30), so they are left as plain text.
func detectAndProcessGIFURLs(content string, messageID string) (string, string) {
	var attachmentHTML string
	var attachmentText string

	// Check for Giphy URLs (basic support - Giphy API is more complex)
	if matches := giphyURLPattern.FindAllStringSubmatch(content, -1); len(matches) > 0 {
		for _, match := range matches {
			if len(match) > 1 {
				gifID := match[1]
				discordColor.Printf("Detected Giphy GIF ID: %s\n", gifID)

				// For now, construct the direct media URL (this is a simplified approach)
				gifURL := fmt.Sprintf("https://media.giphy.com/media/%s/giphy.gif", gifID)

				filename, err := downloadDiscordEmbedMedia(gifURL, messageID)
				if err != nil {
					discordColor.Printf("Failed to download Giphy GIF %s: %v\n", gifURL, err)
					// Fallback to direct URL
					attachmentHTML += fmt.Sprintf(`<img src="%s" class="attachment">`, gifURL)
				} else {
					attachmentHTML += fmt.Sprintf(`<img src="attachments/%s" class="attachment">`, filename)
				}

				attachmentText += "[Giphy GIF]"
			}
		}
	}

	// Check for direct Giphy media URLs
	if matches := giphyMediaPattern.FindAllStringSubmatch(content, -1); len(matches) > 0 {
		for _, match := range matches {
			gifURL := match[0]
			discordColor.Printf("Detected direct Giphy media URL: %s\n", gifURL)

			filename, err := downloadDiscordEmbedMedia(gifURL, messageID)
			if err != nil {
				discordColor.Printf("Failed to download Giphy media %s: %v\n", gifURL, err)
				// Fallback to direct URL
				attachmentHTML += fmt.Sprintf(`<img src="%s" class="attachment">`, gifURL)
			} else {
				attachmentHTML += fmt.Sprintf(`<img src="attachments/%s" class="attachment">`, filename)
			}

			attachmentText += "[Giphy GIF]"
		}
	}

	return attachmentHTML, attachmentText
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
	embeds := m.Embeds
	if len(embeds) == 0 && containsGIFServiceURL(content) {
		// Discord unfurls GIF links into embeds shortly after MESSAGE_CREATE;
		// this handler runs in its own goroutine, so blocking here is fine.
		embeds = waitForMessageEmbeds(s, m.ChannelID, m.ID)
	}
	if len(embeds) == 0 {
		gifHTML, gifText := detectAndProcessGIFURLs(content, m.ID)
		if gifHTML != "" {
			content = ""
			attachmentHTML += gifHTML
			attachmentText += gifText
		}
	} else {
		content = ""
		for _, embed := range embeds {
			attachmentText += fmt.Sprintf("[embed %s]", embed.URL)
			// GIF services unfurl to a silent looping MP4 ("gifv"); render it
			// like a GIF rather than a video player
			if embed.Type == discordgo.EmbedTypeGifv && embed.Video != nil && embed.Video.URL != "" {
				filename, err := downloadDiscordEmbedMedia(embed.Video.URL, m.ID)
				if err != nil {
					discordColor.Printf("Failed to download embed video %s: %v\n", embed.Video.URL, err)
					// If download fails, just play the video directly from the URL
					attachmentHTML += fmt.Sprintf(`<video src="%s" class="attachment" autoplay loop muted playsinline></video>`, embed.Video.URL)
				} else {
					attachmentHTML += fmt.Sprintf(`<video src="attachments/%s" class="attachment" autoplay loop muted playsinline></video>`, filename)
				}
				continue
			}
			// Check if the embed has an image (like GIFs)
			if embed.Image != nil && embed.Image.URL != "" {
				// Download the embed image (GIF)
				filename, err := downloadDiscordEmbedMedia(embed.Image.URL, m.ID)
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

// downloadDiscordAttachment downloads a Discord attachment to the ./attachments/ directory.
// Returns filename and error.
func downloadDiscordAttachment(attachment *discordgo.MessageAttachment, discordMessageID string) (string, error) {
	// Create filename with the format {discord_message_id}_{attachment_name}
	filename := fmt.Sprintf("%s_%s", discordMessageID, attachment.Filename)
	localPath := filepath.Join("attachments", filename)

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

// downloadDiscordEmbedMedia downloads a Discord embed's media (GIF, MP4, etc.) to the ./attachments/ directory.
// Returns filename and error.
func downloadDiscordEmbedMedia(mediaURL string, discordMessageID string) (string, error) {
	// Extract filename from URL or create one
	parsedURL, err := url.Parse(mediaURL)
	if err != nil {
		return "", fmt.Errorf("failed to parse embed media URL: %w", err)
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
	localPath := filepath.Join("attachments", filename)

	// Download the file
	resp, err := http.Get(mediaURL)
	if err != nil {
		return "", fmt.Errorf("failed to download embed media: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to download embed media, status: %d", resp.StatusCode)
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
		return "", fmt.Errorf("failed to save embed media: %w", err)
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
