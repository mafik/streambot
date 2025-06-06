package main

import (
	"os/exec"
	"streambot/backoff"
	"strings"
	"time"

	"github.com/fatih/color"
	"golang.org/x/sys/windows"
)

const vlcDir = "C:\\Program Files\\VideoLAN\\VLC\\"
const vlcExecutable = "vlc.exe"
const musicPath = "C:\\Users\\User\\Music\\Music"

func findVlc() (windows.HWND, error) {
	hwnd, err := FindWindow("VLC media player")
	return windows.HWND(hwnd), err
}

func VlcMonitor(audioMessages chan string) {
	audioMessages <- "Connecting..."
	col := color.New(color.FgCyan)
	vlcBackoff := backoff.Backoff{
		Color:       col,
		Description: "VLC Monitor",
	}
	for {
		vlcBackoff.Attempt()

		vlc, err := findVlc()
		if err != nil {
			cmd := exec.Command(vlcDir+vlcExecutable, musicPath)
			cmd.Dir = vlcDir
			err := cmd.Start()
			if err != nil {
				col.Println("Couldn't start VLC:", err)
				continue
			}
			col.Println("Starting VLC...")
			// wait up to 30 seconds for OBS to start
			for i := 0; i < 30; i++ {
				vlc, _ = findVlc()
				if vlc != 0 {
					break
				}
				time.Sleep(time.Second)
			}
			if vlc != 0 {
				col.Println("Started VLC")
			} else {
				col.Println("Couldn't start VLC")
				continue
			}
		}
		if vlc == 0 {
			continue
		}

		lastAudioMessage := ""
		for {
			// Get the title of the VLC window
			wmName, err := GetWindowTitle(vlc)
			if err != nil {
				col.Println("Couldn't get WM_NAME:", err)
				break
			}
			vlcBackoff.Success()
			// if an extension is present we know that there is some song playing,
			// otherwise we report "No song playing"
			audioMessage := "No song playing"
			dotIndex := strings.LastIndex(wmName, ".")
			if dotIndex != -1 {
				audioMessage = wmName[:dotIndex]
				audioMessage = strings.TrimSpace(audioMessage)
			}
			if audioMessage != lastAudioMessage {
				lastAudioMessage = audioMessage
				audioMessages <- audioMessage
			}
			time.Sleep(1 * time.Second)
		}
	}
}
