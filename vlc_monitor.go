package main

import (
	"errors"
	"streambot/backoff"
	"strings"
	"time"

	"github.com/fatih/color"
	"golang.org/x/crypto/ssh"
)

func VlcMonitor(audioMessages chan string) {
	audioMessages <- "Connecting..."
	col := color.New(color.FgCyan)
	sshBackoff := backoff.Backoff{
		Color:       col,
		Description: "VLC SSH",
	}
	for {
		sshBackoff.Attempt()
		vlcSsh, err := NewSSH("vr:17275")
		if err != nil {
			col.Println("Couldn't connect to vr:", err)
			continue
		}
		func() {
			var vlcPid string
			defer vlcSsh.Close()
			defer func() {
				if vlcPid != "" {
					vlcSsh.Exec("kill " + vlcPid)
				}
				audioMessages <- "No song playing"
			}()
			vlcBackoff := backoff.Backoff{
				Color:       col,
				Description: "VLC Monitor",
			}
			for {
				vlcBackoff.Attempt()
				output, err := vlcSsh.Exec("DISPLAY=:0 xdotool search --all --name \"VLC media player\"")
				// Find ID of the VLC window
				if err != nil {
					if _, ok := errors.Unwrap(err).(*ssh.ExitError); ok {
						// Start VLC and wait a couple of seconds for it to appear
						if output != "" {
							col.Println(output)
						}
						col.Println("VLC window not found, starting VLC...")
						vlcPid, err = vlcSsh.Exec("DISPLAY=:0 vlc /home/maf/Pulpit/Streaming/Music >/dev/null 2>&1 & ; echo $last_pid")
						if err != nil {
							col.Println("Couldn't start vlc:", err)
							break
						}
						vlcPid = strings.TrimSpace(vlcPid)
						col.Println("Started VLC with PID", vlcPid)
						continue
					} else {
						col.Printf("Couldn't run xdotool: %#v\n", err)
						break
					}
				}
				lines := strings.Split(strings.TrimSpace(output), "\n")
				if len(lines) == 0 {
					col.Println("xdotool didn't return any lines but also didn't error out (bug)")
					continue
				}
				// VLC may create multiple windows - one for the tray icon and one for the main window
				// We don't care which one we take because they're both have the same title
				windowID := lines[0]

				lastAudioMessage := ""
				for {
					// Get the title of the VLC window
					wmName, err := vlcSsh.Exec("DISPLAY=:0 xprop -id " + windowID + " WM_NAME")
					if err != nil {
						col.Println("Couldn't get WM_NAME:", err)
						break
					}
					firstQuote := strings.Index(wmName, "\"")
					lastQuote := strings.LastIndex(wmName, "\"")
					if firstQuote == -1 || lastQuote == -1 {
						col.Println("xprop returned unexpected output:", wmName)
						break
					}
					vlcBackoff.Success()
					wmName = wmName[firstQuote+1 : lastQuote]
					// if an extension is present we know that there is some song playing,
					// otherwise we report "No song playing"
					audioMessage := "No song playing"
					dotIndex := strings.LastIndex(wmName, ".")
					if dotIndex != -1 {
						audioMessage = wmName[:dotIndex]
						audioMessage = strings.TrimSpace(audioMessage)
					}
					if audioMessage != lastAudioMessage {
						col.Println(" ", audioMessage)
						lastAudioMessage = audioMessage
						audioMessages <- audioMessage
					}
					time.Sleep(1 * time.Second)
				}

			}
		}() // defer ssh.Close()
	} // for
}
