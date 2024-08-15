package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"time"

	"github.com/fatih/color"
	externalip "github.com/glendc/go-external-ip"
)

type Alert struct {
	HTML   string `json:"html"`
	onPlay func() // function to call when the alert is played (optional)
}

type ChatEntry struct {
	Author          UserVariant `json:"author,omitempty"`
	OriginalMessage string      `json:"original_message"`
	HTML            string      `json:"html,omitempty"`
	ttsMsg          string
	timestamp       time.Time
	terminalMsg     string
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
