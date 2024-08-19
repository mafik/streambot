package main

import (
	"bytes"
	"streambot/backoff"
	"time"

	"github.com/ebitengine/oto/v3"
	"github.com/fatih/color"
)

var AudioPlayerChannel = make(chan interface{}, 20)

var audioPlayerColor = color.New(color.FgGreen)

type PlayMessage struct {
	wavData  []byte // first 44 bytes (WAV header) are ignored
	prePlay  func() // optional function to run before playing (blocks audio playback)
	postPlay func() // optional function to run after playing (blocks audio playback)
	author   *User
}

func WAVDuration(wav []byte) time.Duration {
	const bytesPerSecond = 44100 * 2
	return time.Duration(len(wav)-44) * time.Second / time.Duration(bytesPerSecond)
}

func WaitForMicSilence() {
	if !MicIsSilent.Load() {
		audioPlayerColor.Println("Audio Player waiting for mic silence...")
		waitStart := time.Now()
		// wait for the mic to be silent
		for !MicIsSilent.Load() {
			time.Sleep(time.Millisecond * 100)
		}
		audioPlayerColor.Println("Resuming playback after", time.Since(waitStart))
	}
}

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
				case PlayMessage:
					samples := t.wavData[44:] // remove WAV header
					player := otoCtx.NewPlayer(bytes.NewReader(samples))
					WaitForMicSilence()
					if t.prePlay != nil {
						t.prePlay()
					}
					player.Play()
					for player.IsPlaying() {
						if t.author != nil && IsMuted(*t.author) {
							player.Pause()
							break
						}
						time.Sleep(time.Millisecond)
					}
					if t.postPlay != nil {
						t.postPlay()
					}
				default:
					audioPlayerColor.Printf("Player received unknown message type: %T\n", t)
				}
			}
		}
	}
}
