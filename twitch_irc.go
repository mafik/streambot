package main

import (
	"fmt"
	"html"
	"path"
	"streambot/backoff"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/gempir/go-twitch-irc"
	"github.com/nicklaw5/helix/v2"
)

const twitchBotUsername = "bot_maf"
const twitchBroadcasterUsername = "maf_pl"

var twitchBotID string
var twitchBroadcasterID string

var TwitchIRCChannel = make(chan interface{})
var twitchEmotes map[string]string

func getTwitchEmotes() *map[string]string {
	if twitchEmotes == nil {
		emotesChannel := make(chan map[string]string)
		TwitchHelixChannel <- func(client *helix.Client) {
			resp, err := client.GetGlobalEmotes()
			emotes := make(map[string]string)
			if err == nil {
				for _, emote := range resp.Data.Emotes {
					emotes[emote.Name] = emote.Images.Url4x
				}
			}
			users, err := client.GetUsers(&helix.UsersParams{Logins: []string{twitchBroadcasterUsername}})
			if err == nil {
				id := users.Data.Users[0].ID
				resp, err = client.GetChannelEmotes(&helix.GetChannelEmotesParams{BroadcasterID: id})
				if err == nil {
					for _, emote := range resp.Data.Emotes {
						emotes[emote.Name] = emote.Images.Url4x
					}
				}
			} else {
				fmt.Println("Couldn't get Twitch user ID:", err)
			}
			emotesChannel <- emotes
		}
		twitchEmotes = <-emotesChannel
		for emote, url := range twitchEmotes {
			escapedEmote := html.EscapeString(emote)
			if escapedEmote != emote {
				twitchEmotes[escapedEmote] = url
			}
		}
	}
	return &twitchEmotes
}

// Goroutine for the Twitch client
func TwitchIRCBot() {
	color := color.New(color.FgMagenta)
	backoff := backoff.Backoff{
		Color:       color,
		Description: "Twitch IRC",
	}
	for {
		backoff.Attempt()

		emotes := getTwitchEmotes()

		ircToken, err := ReadStringFromFile(path.Join(baseDir, "secrets", "twitch_irc_token.txt"))
		if err != nil {
			color.Println("Couldn't read twitch_irc_token.txt:", err)
			continue
		}
		irc := twitch.NewClient(twitchBotUsername, ircToken)
		irc.IdlePingInterval = 5 * time.Second
		irc.PongTimeout = 10 * time.Second
		irc.OnPingSent(func() {
			Webserver.Call("Ping", "Twitch")
		})
		irc.OnPongReceived(func() {
			Webserver.Call("Pong", "Twitch")
		})
		irc.OnConnect(func() {
			color.Println("Connected to Twitch IRC server")
		})
		irc.OnNewMessage(func(channel string, user twitch.User, message twitch.Message) {
			entry := ChatEntry{
				Author:       user.DisplayName,
				Message:      message.Text,
				Source:       "Twitch",
				AuthorColor:  user.Color,
				TwitchUserId: user.UserID,
			}

			entry.terminalMsg = fmt.Sprintf("  %s: %s\n", entry.Author, entry.Message)
			entry.Message = html.EscapeString(entry.Message)

			i := 0
			for i < len(entry.Message) {
				replaced := false
				if i == 0 || entry.Message[i-1] == ' ' {
					for emote, url := range *emotes {
						if i+len(emote) >= len(entry.Message)-1 || entry.Message[i+len(emote)] == ' ' {
							if strings.HasPrefix(entry.Message[i:], emote) {
								tag := `<img src="` + url + `" class="emoji" />`
								entry.Message = entry.Message[:i] + tag + entry.Message[i+len(emote):]
								i += len(tag)
								replaced = true
								break
							}
						}
					}
				}
				if !replaced {
					i++
				}
			}

			MainChannel <- entry
		})
		irc.Join(twitchBroadcasterUsername)
		err = irc.Connect()
		if err != nil {
			color.Println("IRC error:", err)
			continue
		}
		color.Println("Twitch IRC server exited the main loop ?!")
	}
}
