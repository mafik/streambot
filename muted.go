package main

import (
	"bufio"
	"fmt"
	"os"
	"path"
)

var muted = make(map[string]bool)

func readMuted() map[string]bool {
	muted := make(map[string]bool)
	path := path.Join(baseDir, "muted.txt")
	file, err := os.Open(path)
	if err != nil {
		warn_color.Println("Couldn't open muted.txt:", err)
		return muted
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		muted[scanner.Text()] = true
	}
	if err := scanner.Err(); err != nil {
		warn_color.Println("Couldn't read muted.txt:", err)
	}
	return muted
}

func saveMuted(muted map[string]bool) {
	path := path.Join(baseDir, "muted.txt")
	file, err := os.Create(path)
	if err != nil {
		warn_color.Println("Couldn't create muted.txt:", err)
		return
	}
	defer file.Close()
	for user := range muted {
		file.WriteString(user + "\n")
	}
}

func ToggleMuted(args ...string) {
	if len(args) != 1 {
		warn_color.Println("ToggleMuted: wrong number of arguments:", args)
		return
	}
	user := args[0]

	MainChannel <- func() {
		if muted[user] {
			chat_color.Println("Unmuting", user)
			delete(muted, user)
			MainOnChatEntry(ChatEntry{
				Source:  "Bot",
				Message: fmt.Sprintf(` <img class="emoji" src="static/unmuted.svg"> <strong>%s</strong>`, user),
				skipTTS: true,
			})
		} else {
			chat_color.Println("Muting", user)
			muted[user] = true
			MainOnChatEntry(ChatEntry{
				Source:  "Bot",
				Message: fmt.Sprintf(` <img class="emoji" src="static/muted.svg"> <strong>%s</strong>`, user),
				skipTTS: true,
			})
		}
		saveMuted(muted)
	}
}
