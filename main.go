package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	"github.com/fatih/color"
	externalip "github.com/glendc/go-external-ip"
	"github.com/nicklaw5/helix/v2"
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
	ttsMsg           string
	timestamp        time.Time
	terminalMsg      string
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
			ctx := context.Background()
			liveChatMessageID := ""
			// This is a very roundabout way of getting the message ID for deletion.
			// It is necessary because the hacky way of getting YouTube chat reports different message IDs.
			// It might be better to use the official API in tandem with the hacky way - when the hacky way reports a new message, the official API could be used to read it.
			// This way the official API loop would sleep most of the time and not use up the quota.
			// TODO(when the issues pile up): implement this approach
			yt.LiveChatMessages.List(youtubeLiveChatID, []string{"id", "snippet"}).MaxResults(2000).Pages(ctx, func(resp *youtube.LiveChatMessageListResponse) error {
				for _, msg := range resp.Items {
					if msg.Snippet.DisplayMessage == t.OriginalMessage {
						liveChatMessageID = msg.Id
						return fmt.Errorf("target message found")
					}
				}
				if len(resp.Items) < 2000 {
					return fmt.Errorf("no new messages found") // error seems to be necessary to stop the loop
				}
				return nil
			})
			if liveChatMessageID == "" {
				warn_color.Println("Couldn't delete Youtube message - unable to locate message in chat:", t.OriginalMessage)
				return nil
			}
			return yt.LiveChatMessages.Delete(liveChatMessageID).Do()
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
		// try writing to TTSChannel (ignore if full)
		select {
		case TTSChannel <- t:
		default:
			warn_color.Println("TTS is busy, dropping message")
		}
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
