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

// https://dev.twitch.tv/docs/eventsub/eventsub-subscription-types/#channelraid
type TwitchRaidNotification struct {
	Payload struct {
		Event struct {
			FromBroadcasterUserID    string `json:"from_broadcaster_user_id"`
			FromBroadcasterUserLogin string `json:"from_broadcaster_user_login"`
			FromBroadcasterUserName  string `json:"from_broadcaster_user_name"`
			Viewers                  int    `json:"viewers"`
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
					err = TwitchEventsSubscribeKnown(welcome_msg.Payload.Session.ID)
					if err != nil {
						twitchColor.Println("Twitch EventSub configure ERROR:", err)
						return
					}
				case "notification":
					var generic_notification TwitchNotification
					err = json.Unmarshal(bytes, &generic_notification)
					if err != nil {
						twitchColor.Println("Twitch EventSub cannot unmarshal notification:", err, string(bytes))
						return
					}
					switch generic_notification.Payload.Subscription.Type {
					case "channel.follow":
						var notification TwitchFollowNotification
						err = json.Unmarshal(bytes, &notification)
						if err != nil {
							twitchColor.Println("Twitch EventSub cannot unmarshal follow:", err, string(bytes))
							return
						}
						event := notification.Payload.Event
						TTSChannel <- Alert{
							HTML: fmt.Sprintf(`<div class="big">%s</div>Just followed on Twitch!`, event.UserName),
							onPlay: func() {
								MainChannel <- ChatEntry{
									Message:      fmt.Sprintf("<strong>%s</strong> 💜 just followed on Twitch!", event.UserName),
									terminalMsg:  fmt.Sprintf("  %s 💜 just followed on Twitch!\n", event.UserName),
									Source:       "Twitch",
									TwitchUserId: event.UserID,
									skipTTS:      true,
								}
							},
						}
					case "channel.raid":
						var notification TwitchRaidNotification
						err = json.Unmarshal(bytes, &notification)
						if err != nil {
							twitchColor.Println("Twitch EventSub cannot unmarshal raid:", err, string(bytes))
							return
						}
						event := notification.Payload.Event
						TTSChannel <- Alert{
							HTML: fmt.Sprintf(`<div class="big">%s</div>is raiding with %d viewers!`, event.FromBroadcasterUserName, event.Viewers),
							onPlay: func() {
								MainChannel <- ChatEntry{
									Message:      fmt.Sprintf("<strong>%s</strong> 🚨 is raiding with %d viewers!", event.FromBroadcasterUserName, event.Viewers),
									terminalMsg:  fmt.Sprintf("  %s 🚨 is raiding with %d viewers!\n", event.FromBroadcasterUserName, event.Viewers),
									Source:       "Twitch",
									TwitchUserId: event.FromBroadcasterUserID,
									skipTTS:      true,
								}
							},
						}
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

// See https://dev.twitch.tv/docs/eventsub/eventsub-subscription-types/
func TwitchEventSubscribe(sessionID, subscriptionType, version string, condition helix.EventSubCondition) <-chan error {
	errChan := make(chan error)
	TwitchHelixChannel <- func(client *helix.Client) {
		followSub, err := client.CreateEventSubSubscription(&helix.EventSubSubscription{
			Type:      subscriptionType,
			Version:   version,
			Condition: condition,
			Transport: helix.EventSubTransport{
				Method:    "websocket",
				SessionID: sessionID,
			},
		})
		if err != nil {
			errChan <- err
			return
		}
		if followSub.Error != "" {
			errChan <- fmt.Errorf("Twitch event subscription for %s:%s failed %s", subscriptionType, version, followSub.ErrorMessage)
			return
		}
		errChan <- nil
	}
	return errChan
}

func TwitchEventsSubscribeKnown(sessionID string) error {
	err := <-TwitchEventSubscribe(sessionID, "channel.follow", "2", helix.EventSubCondition{
		BroadcasterUserID: twitchBroadcasterID,
		ModeratorUserID:   twitchBotID,
	})
	if err != nil {
		return err
	}
	err = <-TwitchEventSubscribe(sessionID, "channel.raid", "1", helix.EventSubCondition{
		ToBroadcasterUserID: twitchBroadcasterID,
	})
	if err != nil {
		return err
	}
	return nil
}
