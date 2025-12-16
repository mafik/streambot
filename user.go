package main

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"path"
)

type User struct {
	TwitchUser        *TwitchUser  `json:"twitch,omitempty"`
	YouTubeUser       *YouTubeUser `json:"youtube,omitempty"`
	BotUser           *BotUser     `json:"bot,omitempty"`
	DiscordUser       *DiscordUser `json:"discord,omitempty"`
	Ticket            string       `json:"ticket,omitempty"`
	Voice             string       `json:"voice,omitempty"`
	NamePronunciation string       `json:"name_pronunciation,omitempty"`
	websockets        []*WebsocketClient
}

var TwitchIndex = map[string]*User{}
var YouTubeIndex = map[string]*User{}
var DiscordIndex = map[string]*User{}
var PasswordIndex = map[string]*User{}
var TicketIndex = map[string]*User{}

var usersPath = path.Join(baseDir, "secrets", "users.json")

func SaveUsers() error {
	var usersToSave map[string]User = map[string]User{}
	for password, user := range PasswordIndex {
		worthSaving := user.TwitchUser != nil || user.YouTubeUser != nil || user.DiscordUser != nil || user.Voice != "" || user.NamePronunciation != ""
		if worthSaving {
			// skip non-essential data
			userToSave := *user
			userToSave.websockets = nil
			userToSave.Ticket = ""
			usersToSave[password] = userToSave
		}
	}
	bytes, err := json.Marshal(usersToSave)
	if err != nil {
		return fmt.Errorf("couldn't marshal users to save: %w", err)
	}
	err = WriteStringToFile(usersPath, string(bytes))
	if err != nil {
		return fmt.Errorf("couldn't write users to file: %w", err)
	}
	return nil
}

func LoadUsers() error {
	usersStr, err := ReadStringFromFile(usersPath)
	if err != nil {
		return fmt.Errorf("couldn't read users file: %w", err)
	}
	err = json.Unmarshal([]byte(usersStr), &PasswordIndex)
	if err != nil {
		return fmt.Errorf("couldn't unmarshal users: %w", err)
	}
	for _, user := range PasswordIndex {
		user.IssueTicket()
		if user.TwitchUser != nil {
			if _, found := TwitchIndex[user.TwitchUser.Key()]; found {
				user.TwitchUser = nil
			} else {
				TwitchIndex[user.TwitchUser.Key()] = user
			}
		}
		if user.YouTubeUser != nil {
			if _, found := YouTubeIndex[user.YouTubeUser.Key()]; found {
				user.YouTubeUser = nil
			} else {
				YouTubeIndex[user.YouTubeUser.Key()] = user
			}
		}
		if user.DiscordUser != nil {
			if _, found := DiscordIndex[user.DiscordUser.Key()]; found {
				user.DiscordUser = nil
			} else {
				DiscordIndex[user.DiscordUser.Key()] = user
			}
		}
	}
	return nil
}

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

func (u User) LoadSettings() *User {
	if u.TwitchUser != nil {
		if userConfig, found := TwitchIndex[u.TwitchUser.Key()]; found {
			return userConfig
		}
	} else if u.YouTubeUser != nil {
		if userConfig, found := YouTubeIndex[u.YouTubeUser.Key()]; found {
			return userConfig
		}
	} else if u.DiscordUser != nil {
		if userConfig, found := DiscordIndex[u.DiscordUser.Key()]; found {
			return userConfig
		}
	}
	return &u
}

func (u User) DisplayName() string {
	if u.TwitchUser != nil {
		return u.TwitchUser.DisplayName()
	} else if u.YouTubeUser != nil {
		return u.YouTubeUser.DisplayName()
	} else if u.DiscordUser != nil {
		return u.DiscordUser.DisplayName()
	} else if u.BotUser != nil {
		return u.BotUser.DisplayName()
	}
	return ""
}

func (u User) GetNamePronunciation() string {
	if u.NamePronunciation != "" {
		return u.NamePronunciation
	}
	return u.DisplayName()
}

func (u User) Key() string {
	if u.TwitchUser != nil {
		return u.TwitchUser.Key()
	} else if u.YouTubeUser != nil {
		return u.YouTubeUser.Key()
	} else if u.DiscordUser != nil {
		return u.DiscordUser.Key()
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
	} else if u.DiscordUser != nil && u.DiscordUser.Avatar != "" {
		avatar_url = fmt.Sprintf("https://cdn.discordapp.com/avatars/%s/%s.png", u.DiscordUser.ID, u.DiscordUser.Avatar)
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
	DISCORD_KEY_PREFIX = "Discord:"
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
