package main

import (
	"bufio"
	"encoding/json"
	"os"
	"path"
	"sync"
)

var muted *sync.Map

func IsMuted(user User) bool {
	_, is_muted := muted.Load(user.Key())
	return is_muted
}

func readMuted() *sync.Map {
	muted := &sync.Map{}
	path := path.Join(baseDir, "muted.txt")
	file, err := os.Open(path)
	if err != nil {
		warn_color.Println("Couldn't open muted.txt:", err)
		return muted
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var author User
		err = json.Unmarshal([]byte(scanner.Text()), &author)
		if err != nil {
			warn_color.Println("Couldn't unmarshal muted user:", err)
			continue
		}
		muted.Store(author.Key(), author)
	}
	if err := scanner.Err(); err != nil {
		warn_color.Println("Couldn't read muted.txt:", err)
	}
	return muted
}

func saveMuted(muted *sync.Map) {
	path := path.Join(baseDir, "muted.txt")
	file, err := os.Create(path)
	if err != nil {
		warn_color.Println("Couldn't create muted.txt:", err)
		return
	}
	defer file.Close()
	muted.Range(func(key, value interface{}) bool {
		bytes, err := json.Marshal(value)
		if err != nil {
			warn_color.Println("Couldn't marshal muted user:", err)
			return true
		}
		file.WriteString(string(bytes) + "\n")
		return true
	})
}

const BOT_ICON = `<img src="bot.svg" class="emoji">`
const MUTED_ICON = `<img src="muted.svg" class="emoji">`
const UNMUTED_ICON = `<img src="unmuted.svg" class="emoji">`

func ToggleMuted(c *WebsocketClient, args ...json.RawMessage) {
	if !c.admin {
		return
	}
	if len(args) != 1 {
		warn_color.Println("ToggleMuted: wrong number of arguments:", args)
		return
	}
	var user User
	err := json.Unmarshal(args[0], &user)
	if err != nil {
		warn_color.Println("ToggleMuted: couldn't unmarshal user:", err)
		return
	}

	key := user.Key()
	username := user.DisplayName()
	_, is_muted := muted.Load(key)
	if is_muted {
		chat_color.Println("Unmuting", username)
		muted.Delete(key)
		MainChannel <- ChatEntry{
			Author: user,
			HTML:   BOT_ICON + ` ` + UNMUTED_ICON + ` ` + user.HTML(),
		}
	} else {
		chat_color.Println("Muting", username)
		user.BotUser = nil
		muted.Store(key, user)
		user.BotUser = &BotUser{}
		MainChannel <- ChatEntry{
			Author: user,
			HTML:   BOT_ICON + ` ` + MUTED_ICON + ` ` + user.HTML(),
		}
	}
	saveMuted(muted)
}
