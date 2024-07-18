package main

import (
	"fmt"
	"path"
	"streambot/backoff"

	"github.com/fatih/color"
	"github.com/nicklaw5/helix/v2"
)

var accessTokenPath = path.Join(baseDir, "secrets", "twitch_access_token.txt")
var refreshTokenPath = path.Join(baseDir, "secrets", "twitch_refresh_token.txt")

func OnUserAccessTokenRefreshed(newAccessToken, newRefreshToken string) {
	WriteStringToFile(accessTokenPath, newAccessToken)
	WriteStringToFile(refreshTokenPath, newRefreshToken)
}

var TwitchHelixChannel = make(chan interface{})

func TwitchHelixBot() {
	color := color.New(color.FgMagenta)
	backoff := backoff.Backoff{
		Color:       color,
		Description: "Twitch IRC",
	}
	for {
		backoff.Attempt()
		clientID, err := ReadStringFromFile(path.Join(baseDir, "secrets", "twitch_client_id.txt"))
		if err != nil {
			color.Println(err)
			continue
		}
		clientSecret, err := ReadStringFromFile(path.Join(baseDir, "secrets", "twitch_client_secret.txt"))
		if err != nil {
			color.Println(err)
			continue
		}
		accessToken, err := ReadStringFromFile(accessTokenPath)
		if err != nil {
			color.Println(err)
			continue
		}
		refreshToken, err := ReadStringFromFile(refreshTokenPath)
		if err != nil {
			color.Println(err)
			continue
		}
		client, err := helix.NewClient(&helix.Options{
			ClientID:        clientID,
			ClientSecret:    clientSecret,
			UserAccessToken: accessToken,
			RefreshToken:    refreshToken,
		})
		if err != nil {
			fmt.Println(err)
			continue
		}
		client.OnUserAccessTokenRefreshed(OnUserAccessTokenRefreshed)
		for msg := range TwitchHelixChannel {
			switch t := msg.(type) {
			case func(*helix.Client):
				t(client)
			}
		}
	}
}
