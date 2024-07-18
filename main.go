package main

import (
	"bufio"
	"encoding/json"
	"os"

	"github.com/fatih/color"
)

type ChatEntry struct {
	Author      string `json:"author,omitempty"`
	Message     string `json:"message"`
	Source      string `json:"source,omitempty"`
	AuthorColor string `json:"author_color,omitempty"`
	terminalMsg string
	skipTTS     bool
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
	if !t.skipTTS {
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

func main() {
	go ObsGaze("Main", "Gaze")

	go TwitchHelixBot()
	go TwitchIRCBot()
	go YouTubeBot()
	go AudioPlayer()
	go OBS()

	lastAudioMessage := ""
	audioMessages := make(chan string)
	go VlcMonitor(audioMessages)

	go TTS()

	// Read the ten last lines from "chat_log.txt"
	var err error
	chat_log, err = ReadLastChatLog()
	if err != nil {
		warn_color.Println("Error while reading chat_log.txt:", err)
	}

	newWebsocketClients := make(chan *WebsocketClient, 16)

	Webserver = StartWebserver(newWebsocketClients)

	for {
		select {
		case audioMessage := <-audioMessages:
			lastAudioMessage = audioMessage
			Webserver.Call("SetAudioMessage", audioMessage)
		case client := <-newWebsocketClients:
			client.Call("SetAudioMessage", lastAudioMessage)
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
