package main

// Abstract representation of a user from any platform
type User interface {
	// Human-presentable name of the user. May include capitalization, emotes, etc.
	DisplayName() string

	// Globally-unique identifier for the user. Starts with the platform name.
	ID() string
}

type TwitchUser struct {
	TwitchID string
	Login    string // ^[a-zA-Z0-9_]{4,25}$
	Name     string
}

func (u TwitchUser) DisplayName() string {
	return u.Name
}

func (u TwitchUser) ID() string {
	return "Twitch:" + u.TwitchID
}
