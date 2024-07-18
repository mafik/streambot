package main

import (
	"bytes"
	"streambot/backoff"
	"time"

	"github.com/ebitengine/oto/v3"
	"github.com/fatih/color"
)

var AudioPlayerChannel = make(chan interface{}, 10)

var audioPlayerColor = color.New(color.FgGreen)

func AudioPlayer() {
	var backoff = backoff.Backoff{
		Color:       audioPlayerColor,
		Description: "Player",
	}
	for { // retry loop
		backoff.Attempt()
		// initialize things here
		otoOptions := &oto.NewContextOptions{
			SampleRate:   44100,
			ChannelCount: 1,
			Format:       oto.FormatSignedInt16LE,
		}
		otoCtx, otoReadyChan, err := oto.NewContext(otoOptions)
		if err != nil {
			audioPlayerColor.Println("Couldn't initialize audio player:", err)
			continue
		}
		<-otoReadyChan

		for { // work loop
			select {
			case msg := <-AudioPlayerChannel:
				switch t := msg.(type) {
				case []byte:
					t = t[44:] // remove WAV header
					player := otoCtx.NewPlayer(bytes.NewReader(t))
					if !MicIsSilent.Load() {
						audioPlayerColor.Println("Audio Player waiting for mic silence...")
						waitStart := time.Now()
						// wait for the mic to be silent
						for !MicIsSilent.Load() {
							time.Sleep(time.Millisecond * 100)
						}
						audioPlayerColor.Println("Resuming playback after", time.Since(waitStart))
					}
					player.Play()
					for player.IsPlaying() {
						time.Sleep(time.Millisecond)
					}
				default:
					audioPlayerColor.Printf("Player received unknown message type: %#v\n", t)
				}
			}
		}
	}
}
