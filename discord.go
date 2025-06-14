package main

import (
	"encoding/json"
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
var tenorAPIKey string      // Tenor API key from secrets

const DISCORD_ICON = `<img src="discord.svg" class="emoji">`

// Tenor API v2 structures
type TenorResponse struct {
	Results []TenorGIF `json:"results"`
}

type TenorGIF struct {
	ID           string                      `json:"id"`
	Title        string                      `json:"title"`
	MediaFormats map[string]TenorMediaFormat `json:"media_formats"` // Changed from "media" array to "media_formats" map
	ItemURL      string                      `json:"itemurl"`
}

type TenorMediaFormat struct {
	URL      string  `json:"url"`
	Duration float64 `json:"duration"`
	Preview  string  `json:"preview"`
	Dims     []int   `json:"dims"`
	Size     int     `json:"size"`
}

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

// fetchTenorGIF fetches GIF data from Tenor API v2 using the GIF ID
func fetchTenorGIF(gifID string) (*TenorGIF, error) {
	if tenorAPIKey == "" {
		return nil, fmt.Errorf("Tenor API key not configured")
	}

	// Use Tenor API v2 endpoint (now called "posts" instead of "gifs")
	// Include required client_key and country parameters for v2
	apiURL := fmt.Sprintf("https://tenor.googleapis.com/v2/posts?ids=%s&key=%s&client_key=streambot&country=US", gifID, tenorAPIKey)

	resp, err := http.Get(apiURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch from Tenor API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Tenor API returned status %d", resp.StatusCode)
	}

	var tenorResp TenorResponse
	if err := json.NewDecoder(resp.Body).Decode(&tenorResp); err != nil {
		return nil, fmt.Errorf("failed to decode Tenor response: %w", err)
	}

	if len(tenorResp.Results) == 0 {
		return nil, fmt.Errorf("no GIF found with ID %s", gifID)
	}

	return &tenorResp.Results[0], nil
}

// detectAndProcessGIFURLs detects GIF service URLs in content and returns HTML for them
func detectAndProcessGIFURLs(content string, messageID string) (string, string, error) {
	var attachmentHTML string
	var attachmentText string

	// Check for Tenor URLs
	if matches := tenorURLPattern.FindAllStringSubmatch(content, -1); len(matches) > 0 {
		for _, match := range matches {
			if len(match) > 1 {
				gifID := match[1]
				gif, err := fetchTenorGIF(gifID)
				if err != nil {
					discordColor.Printf("Failed to fetch Tenor GIF %s: %v\n", gifID, err)
					continue
				}

				// Use the best quality GIF URL available from v2 media_formats map
				var gifURL string
				if gif.MediaFormats != nil {
					// Try different formats in order of preference
					if gifFormat, exists := gif.MediaFormats["gif"]; exists && gifFormat.URL != "" {
						gifURL = gifFormat.URL
					} else if mp4Format, exists := gif.MediaFormats["mp4"]; exists && mp4Format.URL != "" {
						gifURL = mp4Format.URL
					} else if webmFormat, exists := gif.MediaFormats["webm"]; exists && webmFormat.URL != "" {
						gifURL = webmFormat.URL
					}
				}

				if gifURL != "" {
					// Create a fake embed image structure to reuse existing download logic
					embedImage := &discordgo.MessageEmbedImage{
						URL: gifURL,
					}

					filename, err := downloadDiscordEmbedImage(embedImage, messageID)
					if err != nil {
						discordColor.Printf("Failed to download Tenor GIF %s: %v\n", gifURL, err)
						// Fallback to direct URL
						attachmentHTML += fmt.Sprintf(`<img src="%s" class="attachment" title="%s">`, gifURL, gif.Title)
					} else {
						attachmentHTML += fmt.Sprintf(`<img src="attachments/%s" class="attachment" title="%s">`, filename, gif.Title)
					}

					attachmentText += fmt.Sprintf("[Tenor GIF: %s]", gif.ItemURL)
				}
			}
		}
	}

	// Check for Giphy URLs (basic support - Giphy API is more complex)
	if matches := giphyURLPattern.FindAllStringSubmatch(content, -1); len(matches) > 0 {
		for _, match := range matches {
			if len(match) > 1 {
				gifID := match[1]
				discordColor.Printf("Detected Giphy GIF ID: %s\n", gifID)

				// For now, construct the direct media URL (this is a simplified approach)
				gifURL := fmt.Sprintf("https://media.giphy.com/media/%s/giphy.gif", gifID)

				embedImage := &discordgo.MessageEmbedImage{
					URL: gifURL,
				}

				filename, err := downloadDiscordEmbedImage(embedImage, messageID)
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

			embedImage := &discordgo.MessageEmbedImage{
				URL: gifURL,
			}

			filename, err := downloadDiscordEmbedImage(embedImage, messageID)
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

	return attachmentHTML, attachmentText, nil
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
	if len(m.Embeds) == 0 {
		// Only process URLs if there are no embeds (meaning Discord hasn't processed them yet)
		gifHTML, gifText, err := detectAndProcessGIFURLs(content, m.ID)
		if err != nil {
			discordColor.Printf("Error processing GIF URLs: %v\n", err)
		} else if gifHTML != "" {
			content = ""
			attachmentHTML += gifHTML
			attachmentText += gifText
		}
	} else {
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

// downloadDiscordEmbedImage downloads a Discord embed image (like GIF) to the ./attachments/ directory.
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
	localPath := filepath.Join("attachments", filename)

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

	// Load Tenor API key (optional - if not present, GIF processing will be skipped)
	tenorAPIKey, err = ReadStringFromFile(secretsPath("tenor_api_key.txt"))
	if err != nil {
		discordColor.Println("Tenor API key not found - GIF URL processing will be limited")
		tenorAPIKey = "" // Clear any partial value
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
