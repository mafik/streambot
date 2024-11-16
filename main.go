package main

import (
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"strings"

	"github.com/nicklaw5/helix/v2"
)

var twitchEmojis map[string]string // Map of emoji ID to the actual emoji string

func getTwitchEmotes(client *helix.Client) (map[string]string, error) {
	emojis := make(map[string]string)
	
	// Fetch global emotes
	globalEmotesResp, err := client.GetGlobalEmotes(&helix.GlobalEmotesParams{})
	if err != nil {
		return nil, err
	}
	for _, emote := range globalEmotesResp.Data.Emotes {
		emojis[emote.ID] = emote.Name
	}

	// Fetch channel-specific emotes (replace with your broadcaster ID)
	broadcasterID := "your_broadcaster_id"
	channelEmotesResp, err := client.GetChannelEmotes(&helix.ChannelEmotesParams{
		BroadcasterID: broadcasterID,
	})
	if err != nil {
		return nil, err
	}
	for _, emote := range channelEmotesResp.Data.Emotes {
		emojis[emote.ID] = emote.Name
	}

	return emojis, nil
}

func cleanMessageFromEmojis(message string) string {
	for _, emojiName := range twitchEmojis {
		message = strings.ReplaceAll(message, ":"+emojiName+":", "")
	}
	return message
}

func MainOnChatEntry(event *helix.EventSubEvent) {
	client, err := helix.NewClient(&helix.Options{
		ClientID:        clientID,
		ClientSecret:    clientSecret,
		UserAccessToken: accessToken,
		RefreshToken:    refreshToken,
	})
	if err != nil {
		fmt.Println("Error creating Twitch client:", err)
		return
	}

	twitchEmojis, err = getTwitchEmotes(client)
	if err != nil {
		fmt.Println("Error fetching Twitch emotes:", err)
		return
	}

	switch event.Subscription.Type {
	case "channel.chat":
		var chatEvent helix.EventSubChannelChatMessageEvent
		err := json.Unmarshal(event.Event, &chatEvent)
		if err != nil {
			fmt.Println("Error unmarshalling chat event