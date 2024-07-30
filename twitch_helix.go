package main

import (
	"encoding/json"
	"fmt"
	"net"
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
var twitchWebhookSecret string

func OnUserAccessTokenRefreshed(newAccessToken, newRefreshToken string) {
	twitchColor.Println("User access token refreshed! If this spams the console, visit this URL using bot account to authorize it:", twitchAuthUrl)
	WriteStringToFile(accessTokenPath, newAccessToken)
	WriteStringToFile(refreshTokenPath, newRefreshToken)
}

func IsAuthorized(addr string) bool {
	// TODO: use cookies or something like that to authorize
	ip_str, _, _ := net.SplitHostPort(addr)
	return ip_str == "10.0.0.8" || ip_str == "10.0.0.3" || ip_str == "::1" || ip_str == "10.0.0.27"
}

func OnTwitchAuth(w http.ResponseWriter, r *http.Request) {
	if !IsAuthorized(r.RemoteAddr) {
		w.WriteHeader(401)
		w.Write([]byte("Unauthorized " + r.RemoteAddr))
		return
	}
	code := r.URL.Query().Get("code")
	if code == "" {
		w.WriteHeader(400)
		w.Write([]byte("Missing code"))
		return
	}
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

func Ban(args ...json.RawMessage) {
	var user UserVariant
	err := json.Unmarshal(args[0], &user)
	if err != nil {
		twitchColor.Println("Couldn't unmarshal user:", err)
		return
	}
	if user.TwitchUser != nil {
		TwitchHelixChannel <- func(client *helix.Client) {
			_, err := client.BanUser(&helix.BanUserParams{
				BroadcasterID: twitchBroadcasterID,
				ModeratorId:   twitchBotID,
				Body: helix.BanUserRequestBody{
					Reason: "Banned by the streamer",
					UserId: user.TwitchUser.TwitchID,
				},
			})
			if err != nil {
				twitchColor.Println(err)
				return
			}
			twitchColor.Println("Banned", user.DisplayName())
			user.BotUser = &BotUser{}
			MainChannel <- ChatEntry{
				Author: user,
				HTML:   fmt.Sprintf(BOT_ICON+` 💀 %s`, user.HTML()),
			}
		}
	}
}

var TwitchHelixChannel = make(chan interface{}, 100)

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
			Scopes:       []string{"moderator:manage:banned_users", "moderator:read:followers"},
		})
		WriteStringToFile(path.Join(baseDir, "twitch_auth_url.txt"), twitchAuthUrl)
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

		for msg := range TwitchHelixChannel {
			switch t := msg.(type) {
			case func(*helix.Client):
				t(client)
			}
		}
	}
}
