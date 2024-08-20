package main

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"html"
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

// https://dev.twitch.tv/docs/eventsub/eventsub-subscription-types/#channelchatmessage
type TwitchChatMessageNotification struct {
	Payload struct {
		Event struct {
			BroadcasterUserID    string `json:"broadcaster_user_id"`
			BroadcasterUserName  string `json:"broadcaster_user_name"`
			BroadcasterUserLogin string `json:"broadcaster_user_login"`
			ChatterUserID        string `json:"chatter_user_id"`
			ChatterUserName      string `json:"chatter_user_name"`
			ChatterUserLogin     string `json:"chatter_user_login"`
			MessageID            string `json:"message_id"`
			Message              struct {
				Text      string `json:"text"`
				Fragments []struct {
					Type      string `json:"type"`
					Text      string `json:"text"`
					Cheermote *struct {
						Prefix string `json:"prefix"`
						Bits   int    `json:"bits"`
						Tier   int    `json:"tier"`
					} `json:"cheermote,omitempty"`
					Emote *struct {
						ID         string `json:"id"`
						EmoteSetID string `json:"emote_set_id"`
						OwnerID    string `json:"owner_id"`
						Format     []string
					} `json:"emote,omitempty"`
					Mention *struct {
						UserID    string `json:"user_id"`
						UserName  string `json:"user_name"`
						UserLogin string `json:"user_login"`
					} `json:"mention,omitempty"`
				} `json:"fragments"`
			} `json:"message"`
			MessageType string `json:"message_type,omitempty"`
			Badges      []struct {
				SetID string `json:"set_id"`
				ID    string `json:"id"`
				Info  string `json:"info,omitempty"`
			} `json:"badges,omitempty"`
			Cheer *struct {
				Bits int `json:"bits"`
			} `json:"cheer,omitempty"`
			Color string `json:"color,omitempty"`
			Reply *struct {
				ParentMessageID   string `json:"parent_message_id"`
				ParentMessageBody string `json:"parent_message_body"`
				ParentUserID      string `json:"parent_user_id"`
				ParentUserName    string `json:"parent_user_name"`
				ParentUserLogin   string `json:"parent_user_login"`
				ThreadMessageID   string `json:"thread_message_id"`
				ThreadUserID      string `json:"thread_user_id"`
				ThreadUserName    string `json:"thread_user_name"`
				ThreadUserLogin   string `json:"thread_user_login"`
			} `json:"reply,omitempty"`
			ChannelPointsCustomRewardID string `json:"channel_points_custom_reward_id,omitempty"`
			ChanelPointsAnimationID     string `json:"channel_points_animation_id,omitempty"`
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
		Webserver.Call("Ping", "Twitch")
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
					Webserver.Call("Pong", "Twitch")
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
								author := User{TwitchUser: &TwitchUser{TwitchID: event.UserID, Login: event.UserLogin, Name: event.UserName}, BotUser: &BotUser{}}
								MainChannel <- ChatEntry{
									HTML:        fmt.Sprintf("%s 💜 just followed on Twitch!", author.HTML()),
									terminalMsg: fmt.Sprintf("  %s 💜 just followed on Twitch!\n", author.DisplayName()),
									Author:      author,
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
								author := User{TwitchUser: &TwitchUser{TwitchID: event.FromBroadcasterUserID, Login: event.FromBroadcasterUserLogin, Name: event.FromBroadcasterUserName}, BotUser: &BotUser{}}
								MainChannel <- ChatEntry{
									HTML:        fmt.Sprintf(TWITCH_ICON+" %s 🚨 is raiding with %d viewers!", author.HTML(), event.Viewers),
									terminalMsg: fmt.Sprintf("  %s 🚨 is raiding with %d viewers!\n", author.DisplayName(), event.Viewers),
									Author:      author,
								}
							},
						}
					case "channel.chat.message":
						var chat_message_notification TwitchChatMessageNotification
						err = json.Unmarshal(bytes, &chat_message_notification)
						if err != nil {
							twitchColor.Println("Twitch EventSub cannot unmarshal chat message:", err, string(bytes))
							return
						}
						event := chat_message_notification.Payload.Event
						bytes, _ := json.Marshal(event)

						str_msg := string(bytes)
						twitchColor.Println(str_msg)

						entry := ChatEntry{
							Author: User{
								TwitchUser: &TwitchUser{
									TwitchID: event.ChatterUserID,
									Name:     event.ChatterUserName,
									Color:    event.Color,
									Login:    event.ChatterUserLogin,
								},
							},
							OriginalMessage: event.Message.Text,
							TwitchMessageID: event.MessageID,
						}

						entry.terminalMsg = fmt.Sprintf("  %s: ", entry.Author.DisplayName())
						entry.HTML = fmt.Sprintf(TWITCH_ICON+` %s: `, entry.Author.HTML())
						entry.ttsMsg = ""

						for _, fragment := range event.Message.Fragments {
							switch fragment.Type {
							case "text":
								entry.terminalMsg += fragment.Text
								entry.HTML += html.EscapeString(fragment.Text)
								entry.ttsMsg += fragment.Text
							case "cheermote":
								entry.terminalMsg += fmt.Sprintf("CHEER(prefix=%s, bits=%d tier=%d)", fragment.Cheermote.Prefix, fragment.Cheermote.Bits, fragment.Cheermote.Tier)
								entry.HTML += fmt.Sprintf("TODO: support cheermotes (prefix=%s, bits=%d tier=%d)", fragment.Cheermote.Prefix, fragment.Cheermote.Bits, fragment.Cheermote.Tier)
								entry.ttsMsg += fmt.Sprintf("* Cheered %d bits *", fragment.Cheermote.Bits)
							case "emote":
								entry.terminalMsg += fmt.Sprintf("[%s]", fragment.Text)
								entry.HTML += fmt.Sprintf("<img title=\"%s\" class=\"emoji\" src=\"https://static-cdn.jtvnw.net/emoticons/v2/%s/default/light/1.0\" srcset=\"https://static-cdn.jtvnw.net/emoticons/v2/%s/default/light/1.0 1x,https://static-cdn.jtvnw.net/emoticons/v2/%s/default/light/2.0 2x,https://static-cdn.jtvnw.net/emoticons/v2/%s/default/light/3.0 4x\">", fragment.Text, fragment.Emote.ID, fragment.Emote.ID, fragment.Emote.ID, fragment.Emote.ID)
							case "mention":
								mention := User{
									TwitchUser: &TwitchUser{
										TwitchID: fragment.Mention.UserID,
										Login:    fragment.Mention.UserLogin,
										Name:     fragment.Mention.UserName,
									},
								}
								entry.terminalMsg += fragment.Text
								entry.HTML += "@" + mention.HTML()
								entry.ttsMsg += mention.DisplayName()
							}
						}
						entry.terminalMsg += "\n"

						// Keeping the string-replacement code for the eventual possibility that custom (non-Twitch) emotes will be added
						/*
							i := 0
							for i < len(entry.HTML) {
								replaced := false
								if i == 0 || entry.HTML[i-1] == ' ' {
									for emote, url := range *emotes {
										if i+len(emote) >= len(entry.HTML)-1 || entry.HTML[i+len(emote)] == ' ' {
											if strings.HasPrefix(entry.HTML[i:], emote) {
												tag := `<img src="` + url + `" class="emoji" />`
												entry.HTML = entry.HTML[:i] + tag + entry.HTML[i+len(emote):]
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
						*/

						MainChannel <- entry

					default:
						twitchColor.Println("Twitch EventSub unknown notification type:", generic_notification.Payload.Subscription.Type)
					}

				case "session_keepalive":
					Webserver.Call("Ping", "Twitch")
					Webserver.Call("Pong", "Twitch")
					// nothing to do
				default:
					twitchColor.Println("Twitch EventSub unknown message: ", string(bytes))
				}
			}
		}()
		Webserver.Call("Ping", "Twitch")
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
// Run this only on the TwitchHelix thread!
func TwitchEventSubscribe(client *helix.Client, sessionID, subscriptionType, version string, condition helix.EventSubCondition) error {
	msg := helix.EventSubSubscription{
		Type:      subscriptionType,
		Version:   version,
		Condition: condition,
		Transport: helix.EventSubTransport{
			Method:    "websocket",
			SessionID: sessionID,
		},
	}
	followSub, err := client.CreateEventSubSubscription(&msg)
	if err != nil {

		return err
	}
	if followSub.Error != "" {
		return fmt.Errorf("twitch event subscription for %s:%s failed %s", subscriptionType, version, followSub.ErrorMessage)
	}
	return nil
}

func TwitchEventsSubscribeKnown(sessionID string) error {
	errChan := make(chan error)
	TwitchHelixChannel <- func(client *helix.Client) {
		type Sub struct {
			Type      string
			Version   string
			Condition helix.EventSubCondition
		}

		subs := []Sub{
			{"channel.follow", "2",
				helix.EventSubCondition{
					BroadcasterUserID: twitchBroadcasterID,
					ModeratorUserID:   twitchBotID,
				},
			},
			{"channel.raid", "1",
				helix.EventSubCondition{
					ToBroadcasterUserID: twitchBroadcasterID,
				},
			},
			{"channel.chat.message", "1",
				helix.EventSubCondition{
					BroadcasterUserID: twitchBroadcasterID,
					UserID:            twitchBotID,
					ModeratorUserID:   twitchBotID,
				},
			},
		}
		for _, sub := range subs {
			err := TwitchEventSubscribe(client, sessionID, sub.Type, sub.Version, sub.Condition)
			if err != nil {
				errChan <- err
				return
			}
		}
		errChan <- nil
	}
	return <-errChan
}
