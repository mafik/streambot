package main

import (
	"fmt"
	"net/http"
	"path"
	"streambot/backoff"

	"github.com/fatih/color"
	"github.com/nicklaw5/helix/v2"
)

var accessTokenPath = path.Join(baseDir, "secrets", "twitch_access_token.txt")
var refreshTokenPath = path.Join(baseDir, "secrets", "twitch_refresh_token.txt")
var twitchColor = color.New(color.FgMagenta)
var twitchAuthUrl string

func OnUserAccessTokenRefreshed(newAccessToken, newRefreshToken string) {
	twitchColor.Println("User access token refreshed! If this spams the console, visit this URL using bot account to authorize it:", twitchAuthUrl)
	WriteStringToFile(accessTokenPath, newAccessToken)
	WriteStringToFile(refreshTokenPath, newRefreshToken)
}

func OnTwitchAuth(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	w.WriteHeader(200)
	w.Write([]byte("You can close this tab now."))
	TwitchHelixChannel <- func(client *helix.Client) {
		resp, err := client.RequestUserAccessToken(code)
		if err != nil {
			twitchColor.Println("Couldn't request user access token", err)
			return
		}
		client.SetUserAccessToken(resp.Data.AccessToken)
		client.SetRefreshToken(resp.Data.RefreshToken)
		OnUserAccessTokenRefreshed(resp.Data.AccessToken, resp.Data.RefreshToken)
	}
}

func BanTwitch(args ...string) {
	user_id := args[0]
	user_name := args[1]
	TwitchHelixChannel <- func(client *helix.Client) {
		_, err := client.BanUser(&helix.BanUserParams{
			BroadcasterID: twitchBroadcasterID,
			ModeratorId:   twitchBotID,
			Body: helix.BanUserRequestBody{
				Reason: "Banned by the streamer",
				UserId: user_id,
			},
		})
		if err != nil {
			twitchColor.Println(err)
			return
		}
		twitchColor.Println("Banned", user_name)
		MainChannel <- ChatEntry{
			Source:  "Bot",
			Message: fmt.Sprintf(` 💀 <strong>%s</strong>`, user_name),
			skipTTS: true,
		}
	}
}

var TwitchHelixChannel = make(chan interface{})

func TwitchHelixBot() {
	backoff := backoff.Backoff{
		Color:       twitchColor,
		Description: "Twitch IRC",
	}
	for {
		backoff.Attempt()
		clientID, err := ReadStringFromFile(path.Join(baseDir, "secrets", "twitch_client_id.txt"))
		if err != nil {
			twitchColor.Println(err)
			continue
		}
		clientSecret, err := ReadStringFromFile(path.Join(baseDir, "secrets", "twitch_client_secret.txt"))
		if err != nil {
			twitchColor.Println(err)
			continue
		}
		accessToken, err := ReadStringFromFile(accessTokenPath)
		if err != nil {
			twitchColor.Println(err)
			continue
		}
		refreshToken, err := ReadStringFromFile(refreshTokenPath)
		if err != nil {
			twitchColor.Println(err)
			continue
		}
		client, err := helix.NewClient(&helix.Options{
			ClientID:        clientID,
			ClientSecret:    clientSecret,
			UserAccessToken: accessToken,
			RefreshToken:    refreshToken,
			RedirectURI:     "http://localhost:3447/twitch-auth",
		})
		if err != nil {
			fmt.Println(err)
			continue
		}
		client.OnUserAccessTokenRefreshed(OnUserAccessTokenRefreshed)
		twitchAuthUrl = client.GetAuthorizationURL(&helix.AuthorizationURLParams{
			ResponseType: "code",
			Scopes:       []string{"moderator:manage:banned_users"},
		})

		getUsersResp, err := client.GetUsers(&helix.UsersParams{Logins: []string{twitchBroadcasterUsername, twitchBotUsername}})
		if err != nil {
			twitchColor.Println("Couldn't get user IDs: ", err)
			continue
		}
		for _, user := range getUsersResp.Data.Users {
			if user.Login == twitchBroadcasterUsername {
				twitchBroadcasterID = user.ID
			} else if user.Login == twitchBotUsername {
				twitchBotID = user.ID
			}
		}
		// getEventSubResp, err := client.GetEventSubSubscriptions(&helix.EventSubSubscriptionsParams{})
		// if err != nil {
		// 	fmt.Println(err)
		// 	continue
		// }
		// color.Printf("getEventSubResp: %#v\n", getEventSubResp)
		for msg := range TwitchHelixChannel {
			switch t := msg.(type) {
			case func(*helix.Client):
				t(client)
			}
		}
	}
}
