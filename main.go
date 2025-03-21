package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	"github.com/fatih/color"
	externalip "github.com/glendc/go-external-ip"
	"github.com/nicklaw5/helix/v2"
	"github.com/pemistahl/lingua-go"
	"google.golang.org/api/youtube/v3"
)

type Alert struct {
	HTML   string `json:"html"`
	onPlay func() // function to call when the alert is played (optional)
}

type ChatEntry struct {
	Author           User   `json:"author,omitempty"`
	OriginalMessage  string `json:"original_message"`
	HTML             string `json:"html,omitempty"`
	TwitchMessageID  string `json:"twitch_message_id,omitempty"`
	YouTubeMessageID string `json:"youtube_message_id,omitempty"`
	ID               int    `json:"id,omitempty"`
	ttsMsg           string
	timestamp        time.Time
	terminalMsg      string
	textOnly         string // user-generated text, excluding emotes
}

func (t ChatEntry) TryTTS() {
	select {
	case TTSChannel <- t:
	default:
		fmt.Println("TTS channel is full, dropping message")
	}
}

func (t *ChatEntry) DeleteUpstream() {
	if t.TwitchMessageID != "" {
		TwitchHelixChannel <- func(client *helix.Client) {
			resp, err := client.DeleteChatMessage(&helix.DeleteChatMessageParams{
				BroadcasterID: twitchBroadcasterID,
				ModeratorID:   twitchBotID,
				MessageID:     t.TwitchMessageID,
			})
			if err != nil {
				warn_color.Println("Couldn't delete Twitch message:", err)
			}
			if resp.StatusCode != 204 {
				warn_color.Println("Couldn't delete Twitch message:", resp.StatusCode)
			}
		}
	}
	if t.YouTubeMessageID != "" {
		YouTubeBotChannel <- func(yt *youtube.Service) error {
			return yt.LiveChatMessages.Delete(t.YouTubeMessageID).Do()
		}
	}
}

var chat_color *color.Color = color.New(color.FgWhite).Add(color.Bold)
var warn_color *color.Color = color.New(color.FgYellow)

func MakeChatEntry(json_string string) (ChatEntry, error) {
	var chat_entry ChatEntry
	err := json.Unmarshal([]byte(json_string), &chat_entry)
	return chat_entry, err
}

var chat_log []ChatEntry

const nChatMessages = 20

const x11_display = ":1"

func ReadLastChatLog() ([]ChatEntry, error) {
	file, err := os.Open("chat_log.txt")
	if err != nil {
		return nil, err
	}
	defer file.Close()

	chat_log := make([]ChatEntry, 0)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		chat_entry, err := MakeChatEntry(scanner.Text())
		if err != nil {
			warn_color.Println("Couldn't parse chat entry:", err)
			continue
		}
		chat_log = append(chat_log, chat_entry)
		if len(chat_log) > nChatMessages {
			chat_log = chat_log[1:]
		}
	}

	if err := scanner.Err(); err != nil {
		return chat_log, err
	}
	return chat_log, nil
}

var linguaDetector = lingua.NewLanguageDetectorBuilder().FromLanguages(lingua.English, lingua.Polish).Build()
var lastReminderTime time.Time

func MainOnChatEntry(t ChatEntry) {
	if t.terminalMsg != "" {
		chat_color.Printf("%s", t.terminalMsg)
	}

	if strings.HasPrefix(t.OriginalMessage, "!login ") {
		t.DeleteUpstream()
		iSpace := strings.IndexRune(t.OriginalMessage, ' ')
		if iSpace == -1 {
			// impossible really
			iSpace = len(t.OriginalMessage) - 1
		}
		ticket := t.OriginalMessage[iSpace+1:]
		user, found := TicketIndex[ticket]
		if !found {
			return
		}
		if t.Author.TwitchUser != nil {
			user.TwitchUser = t.Author.TwitchUser
			TwitchIndex[user.TwitchUser.Key()] = user
		}
		if t.Author.YouTubeUser != nil {
			user.YouTubeUser = t.Author.YouTubeUser
			YouTubeIndex[user.YouTubeUser.Key()] = user
		}
		user.IssueTicket() // invalidate the old ticket
		for _, client := range user.websockets {
			client.Call("Welcome", user)
		}
		err := SaveUsers()
		if err != nil {
			warn_color.Println("Couldn't save users:", err)
		}
		return
	}

	msgCount := 0
	for _, chat_entry := range chat_log {
		if chat_entry.Author.Key() == t.Author.Key() {
			msgCount++
		}
	}

	var detectionThreshold float64
	switch msgCount {
	case 0:
		detectionThreshold = 0.7
	case 1:
		detectionThreshold = 0.8
	case 2:
		detectionThreshold = 0.9
	case 3:
		detectionThreshold = 0.95
	default:
		detectionThreshold = 0.99
	}

	if len(t.textOnly) >= 5 {
		polishConfidence := linguaDetector.ComputeLanguageConfidence(t.textOnly, lingua.Polish)
		if polishConfidence > detectionThreshold {
			fmt.Printf("Blocking likely Polish message: \"%s\" (confidence %f)\n", t.textOnly, polishConfidence)
			t.DeleteUpstream()
			currentTime := time.Now()
			if currentTime.Sub(lastReminderTime) > 5*time.Minute {
				languageRemainder := ChatEntry{
					Author: User{
						Voice: "bg3_narrator.wav",
					},
					ttsMsg: fmt.Sprintf("Hello %s. This is a reminder that TTS only works in English. Please use English in chat... Thank you!", t.Author.DisplayName()),
				}
				languageRemainder.TryTTS()
				lastReminderTime = currentTime
			}
			return
		}
	}

	// Assign ID
	id_bytes, err := os.ReadFile("chat_id.txt")
	id_int := 0
	if err == nil {
		fmt.Sscanf(string(id_bytes), "%d", &id_int)
	}
	id_int += 1
	t.ID = id_int
	os.WriteFile("chat_id.txt", []byte(fmt.Sprintf("%d", id_int)), 0644)

	chat_log = append(chat_log, t)
	if len(chat_log) > nChatMessages {
		chat_log = chat_log[1:]
	}
	entryJson, err := json.Marshal(t)
	if err != nil {
		warn_color.Println("Couldn't marshal chat entry:", err)
		return
	}
	err = AppendToFile("chat_log.txt", string(entryJson)+"\n")
	if err != nil {
		warn_color.Println("Couldn't append to chat_log.txt:", err)
	}
	Webserver.Call("OnChatMessage", t)
	if t.ttsMsg != "" {
		t.TryTTS()
	}
}

var MainChannel = make(chan interface{})

var Webserver *WebsocketHub

var publicIP string

var networkSetupDone = make(chan struct{})

// NetworkSetup fills the public IP with the address of our server and redirects the webserverPort
func NetworkSetup() {
	consensus := externalip.DefaultConsensus(nil, nil)
	ip, err := consensus.ExternalIP()
	if err != nil {
		warn_color.Println("Couldn't get external IP:", err)
	}
	publicIP = ip.String()
	println("Public IP:", publicIP)

	server, _ := net.ResolveTCPAddr("tcp", "google.com:80")
	client, _ := net.ResolveTCPAddr("tcp", fmt.Sprintf(":%d", webserverPort))
	conn, err := net.DialTCP("tcp", client, server)
	if err != nil {
		// Failures here are fine. They mean that we already did port redirection before.
		close(networkSetupDone)
		return
	}
	conn.Close()
	close(networkSetupDone)
}

func main() {
	var err error

	newWebsocketClients := make(chan *WebsocketClient, 16)

	Webserver = StartWebserver(newWebsocketClients)

	go NetworkSetup()

	go ObsGaze("Main", "Gaze")

	go TwitchHelixBot()
	go TwitchEventSub()
	go YouTubeBot()
	go AudioPlayer()
	go OBS()

	lastAudioMessage := ""
	audioMessages := make(chan string)
	go VlcMonitor(audioMessages)
	go BarrierMonitor()

	go TTS()

	// Read the ten last lines from "chat_log.txt"
	chat_log, err = ReadLastChatLog()
	if err != nil {
		warn_color.Println("Error while reading chat_log.txt:", err)
	}

	err = LoadUsers()
	if err != nil {
		warn_color.Println("Error while loading users:", err)
	}

	for {
		select {
		case audioMessage := <-audioMessages:
			lastAudioMessage = audioMessage
			Webserver.Call("SetAudioMessage", audioMessage)
		case client := <-newWebsocketClients:
			client.Call("SetAudioMessage", lastAudioMessage)
			client.Call("SetStreamTitle", twitchTitle)
			for _, entry := range chat_log {
				client.Call("OnChatMessage", entry)
			}
		case msg := <-MainChannel:
			switch t := msg.(type) {
			case ChatEntry:
				MainOnChatEntry(t)
			case func():
				t()
			default:
				warn_color.Printf("Main thread received unknown message: %#v\n", msg)
			}
		}
	}
}
