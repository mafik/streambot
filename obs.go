package main

import (
	"math"
	"net/http"
	"os/exec"
	"path"
	"streambot/backoff"
	"sync/atomic"
	"time"

	"github.com/andreykaipov/goobs"
	"github.com/andreykaipov/goobs/api/events"
	"github.com/andreykaipov/goobs/api/events/subscriptions"
	"github.com/fatih/color"
	"github.com/mitchellh/go-ps"
)

var OBSChannel = make(chan any)

var MicIsSilent atomic.Bool

func OBS() {

	col := color.New(color.FgYellow)
	backoff := backoff.Backoff{
		Color:       col,
		Description: "OBS",
	}
	for {
		backoff.Attempt()

		obsPassword, err := ReadStringFromFile(path.Join(baseDir, "secrets", "obs_password.txt"))
		if err != nil {
			col.Println("Couldn't read OBS password:", err)
			continue
		}

		processes, err := ps.Processes()
		if err != nil {
			col.Println("Couldn't list processes:", err)
			// try to continue
		} else {
			found := false
			for _, process := range processes {
				if process.Executable() == "obs64.exe" {
					found = true
					break
				}
			}
			if !found {
				cmd := exec.Command("C:\\Program Files\\obs-studio\\bin\\64bit\\obs64.exe")
				cmd.Dir = "C:\\Program Files\\obs-studio\\bin\\64bit"
				err := cmd.Start()
				if err != nil {
					col.Println("Couldn't start OBS:", err)
					continue
				}
				col.Println("Starting OBS...")
				// wait up to 30 seconds for OBS to start
				for i := 0; i < 30; i++ {
					probe, err := goobs.New("localhost:4455", goobs.WithPassword(obsPassword))
					if err != nil {
						time.Sleep(time.Second)
						continue
					}
					probe.Disconnect()
				}
			}
		}

		obs, err := goobs.New("localhost:4455",
			goobs.WithPassword(obsPassword),
			goobs.WithRequestHeader(http.Header{"User-Agent": []string{"streambot/1.0"}}),
			goobs.WithEventSubscriptions(subscriptions.InputVolumeMeters),
		)
		if err != nil {
			col.Println("Couldn't connect to OBS:", err)
			continue
		}

		backoff.Success()

		lastMicActivity := time.Now()
		for {
			select {
			case msg := <-OBSChannel:
				switch t := msg.(type) {
				case func(*goobs.Client) error:
					err = t(obs)
					if err != nil {
						break
					}
				default:
					col.Println("Unknown message:", t)
				}
			case obsEvent := <-obs.IncomingEvents:
				if obsEvent == nil {
					col.Println("OBS disconnected")
					break
				}
				switch t := obsEvent.(type) {
				case *events.InputVolumeMeters:
					for _, input := range t.Inputs {
						if input.Name != "Mic" {
							continue
						}
						magnitude := max(input.Levels[0][0], input.Levels[1][0])
						magnitudeDb := 20 * math.Log10(magnitude)
						if magnitudeDb > -40 {
							lastMicActivity = time.Now()
							MicIsSilent.Store(false)
						} else if time.Since(lastMicActivity) > 3*time.Second {
							MicIsSilent.Store(true)
						}
					}
				default:
					col.Println("Unknown OBS event:", t)
				}
			}
		}
	}

}
