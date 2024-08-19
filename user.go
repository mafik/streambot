package main

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
)

type User struct {
	TwitchUser  *TwitchUser  `json:"twitch,omitempty"`
	YouTubeUser *YouTubeUser `json:"youtube,omitempty"`
	BotUser     *BotUser     `json:"bot,omitempty"`
	Ticket      string       `json:"ticket,omitempty"`
	websockets  []*WebsocketClient
}

var PasswordIndex = map[string]*User{
	"123qwe": {
		TwitchUser: &TwitchUser{
			TwitchID: "475318376",
			Login:    "maf_pl",
			Name:     "maf",
			Color:    "#6441a5",
		},
	},
}

var TicketIndex = map[string]*User{}

func (u *User) IssueTicket() {
	if u.Ticket != "" {
		delete(TicketIndex, u.Ticket)
	}
	var randomBytes [18]byte
	_, err := rand.Read(randomBytes[:])
	if err != nil {
		fmt.Println("MakeTicket couldn't generate random bytes:", err)
		u.Ticket = ""
		return
	}
	u.Ticket = base64.StdEncoding.EncodeToString(randomBytes[:])
	TicketIndex[u.Ticket] = u
}

func (u *User) EnsureTicket() {
	if u.Ticket == "" {
		u.IssueTicket()
	}
}

func (u User) DisplayName() string {
	if u.TwitchUser != nil {
		return u.TwitchUser.DisplayName()
	} else if u.YouTubeUser != nil {
		return u.YouTubeUser.DisplayName()
	} else if u.BotUser != nil {
		return u.BotUser.DisplayName()
	}
	return ""
}

func (u User) Key() string {
	if u.TwitchUser != nil {
		return u.TwitchUser.Key()
	} else if u.YouTubeUser != nil {
		return u.YouTubeUser.Key()
	} else if u.BotUser != nil {
		return u.BotUser.Key()
	}
	return ""
}

func (u User) HTML() string {
	ret := ""
	avatar_url := ""
	if u.YouTubeUser != nil {
		avatar_url = u.YouTubeUser.AvatarURL
	}
	if avatar_url != "" {
		ret += `<img src="` + avatar_url + `" class="avatar">`
	}
	color := ""
	if u.TwitchUser != nil {
		color = u.TwitchUser.Color
	}
	if color != "" {
		ret += `<strong style="color:` + color + `">`
	} else {
		ret += `<strong>`
	}
	ret += u.DisplayName()
	ret += `</strong>`
	return ret
}

const (
	TWITCH_KEY_PREFIX  = "Twitch:"
	YOUTUBE_KEY_PREFIX = "YouTube:"
	BOT_KEY_PREFIX     = "Bot"
)

type YouTubeUser struct {
	ChannelID string `json:"channel,omitempty"`
	Name      string `json:"name"`
	AvatarURL string `json:"avatar_url,omitempty"`
}

func (u YouTubeUser) DisplayName() string {
	return u.Name
}

func (u YouTubeUser) Key() string {
	return YOUTUBE_KEY_PREFIX + u.ChannelID
}

type BotUser struct {
}

func (u BotUser) DisplayName() string {
	return "Bot"
}

func (u BotUser) Key() string {
	return BOT_KEY_PREFIX
}

type TwitchUser struct {
	TwitchID string `json:"id"`    // ^[0-9]+$
	Login    string `json:"login"` // ^[a-zA-Z0-9_]{4,25}$
	Name     string `json:"name"`
	Color    string `json:"color,omitempty"`
}

func (u TwitchUser) DisplayName() string {
	return u.Name
}

func (u TwitchUser) Key() string {
	return TWITCH_KEY_PREFIX + u.TwitchID
}
