package main

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"streambot/backoff"

	"github.com/gorilla/websocket"
	"github.com/nicklaw5/helix/v2"
)

type TwitchEventSubMetadata struct {
	MessageID        string `json:"message_id"`
	MessageType      string `json:"message_type"`
	MessageTimestamp string `json:"message_timestamp"`
}

type TwitchEventSubMessage struct {
	Metadata TwitchEventSubMetadata `json:"metadata"`
}

type TwitchSessionWelcome struct {
	Metadata TwitchEventSubMetadata `json:"metadata"`
	Payload  struct {
		Session struct {
			ID                      string `json:"id"`
			Status                  string `json:"status"`
			ConnectedAt             string `json:"connected_at"`
			KeepaliveTimeoutSeconds int    `json:"keepalive_timeout_seconds"`
			ReconnectURL            string `json:"reconnect_url"`
			RecoveryURL             string `json:"recovery_url"`
		} `json:"session"`
	} `json:"payload"`
}

type TwitchNotification struct {
	Payload struct {
		Subscription struct {
			Type string `json:"type"`
		} `json:"subscription"`
	} `json:"payload"`
}

// https://dev.twitch.tv/docs/eventsub/eventsub-subscription-types/#channelfollow
type TwitchFollowNotification struct {
	Payload struct {
		Event struct {
			UserID    string `json:"user_id"`
			UserLogin string `json:"user_login"` // lowercase name
			UserName  string `json:"user_name"`  // pretty name
		} `json:"event"`
	} `json:"payload"`
}

func TwitchEventSub() {
	backoff := backoff.Backoff{
		Color:       twitchColor,
		Description: "Twitch EventSub",
	}
	for {
		backoff.Attempt()
		c, _, err := websocket.DefaultDialer.Dial("wss://eventsub.wss.twitch.tv/ws", nil)
		if err != nil {
			twitchColor.Println("dial:", err)
			continue
		}
		func() { // nested function to ensure that defer is called at the right time
			defer c.Close()
			for {
				_, bytes, err := c.ReadMessage()
				if err != nil {
					twitchColor.Println("Twitch EventSub read ERROR:", err)
					return
				}
				var generic_msg TwitchEventSubMessage
				err = json.Unmarshal(bytes, &generic_msg)
				if err != nil {
					twitchColor.Println("Twitch EventSub cannot unmarshal:", err, string(bytes))
					return
				}
				switch generic_msg.Metadata.MessageType {
				case "session_welcome":
					var welcome_msg TwitchSessionWelcome
					err = json.Unmarshal(bytes, &welcome_msg)
					if err != nil {
						twitchColor.Println("Twitch EventSub cannot unmarshal welcome:", err, string(bytes))
						return
					}
					ConfigureEventSub(welcome_msg.Payload.Session.ID)
				case "notification":
					var generic_notification TwitchNotification
					err = json.Unmarshal(bytes, &generic_notification)
					if err != nil {
						twitchColor.Println("Twitch EventSub cannot unmarshal notification:", err, string(bytes))
						return
					}
					switch generic_notification.Payload.Subscription.Type {
					case "channel.follow":
						var follow_notification TwitchFollowNotification
						err = json.Unmarshal(bytes, &follow_notification)
						if err != nil {
							twitchColor.Println("Twitch EventSub cannot unmarshal follow:", err, string(bytes))
							return
						}
						userName := follow_notification.Payload.Event.UserName
						alert := fmt.Sprintf(`<div class="big">%s</div>Just followed on Twitch!`, userName)
						Webserver.Call("ShowAlert", alert)
						twitchColor.Printf("%s follows on Twitch!\n", userName)
					default:
						twitchColor.Println("Twitch EventSub unknown notification type:", generic_notification.Payload.Subscription.Type)
					}

				case "session_keepalive":
					// nothing to do
				default:
					twitchColor.Println("Twitch EventSub unknown message: ", string(bytes))
				}
			}
		}()
	}
}

func GenerateSecret(n int) (string, error) {
	var secret string
	b := make([]byte, n)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	letters := []string{
		"0", "1", "2", "3", "4", "5", "6", "7", "8", "9", "A", "B", "C", "D", "E", "F", "G", "H",
		"I", "J", "K", "L", "M", "N", "O", "P", "Q", "R", "S", "T", "U", "V", "W", "X", "Y", "Z",
		"a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k", "l", "m", "n", "o", "p", "q", "r",
		"s", "t", "u", "v", "w", "x", "y", "z"}
	for i := range b {
		secret += letters[int(b[i])%len(letters)]
	}
	return secret, nil
}

func ConfigureEventSub(sessionID string) {
	// See https://dev.twitch.tv/docs/eventsub/eventsub-subscription-types/
	TwitchHelixChannel <- func(client *helix.Client) {

		followSub, err := client.CreateEventSubSubscription(&helix.EventSubSubscription{
			Type:    "channel.follow",
			Version: "2",
			Condition: helix.EventSubCondition{
				BroadcasterUserID: twitchBroadcasterID,
				ModeratorUserID:   twitchBotID,
			},
			Transport: helix.EventSubTransport{
				Method:    "websocket",
				SessionID: sessionID,
			},
		})
		if err != nil {
			twitchColor.Println("Error in CreateEventSubSubscription", err)
			return
		}
		if followSub.Error != "" {
			twitchColor.Printf("Twitch Follow subscription failed %#v\n", followSub.ErrorMessage)
			return
		}
	}
}
