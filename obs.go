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
	"github.com/andreykaipov/goobs/api/requests/scenes"
	"github.com/fatih/color"
	"github.com/mitchellh/go-ps"
)

var OBSChannel = make(chan any)

var MicIsSilent atomic.Bool
var OBSScene atomic.Value

func GetOBSScene() string {
	x := OBSScene.Load()
	if x == nil {
		return ""
	}
	if s, ok := x.(*string); ok {
		return *s
	}
	return ""
}

func OBSSwitchScene(targetScene string) error {
	errChan := make(chan error)
	OBSChannel <- func(obs *goobs.Client) error {
		_, err := obs.Scenes.SetCurrentProgramScene(&scenes.SetCurrentProgramSceneParams{
			SceneName: &targetScene,
		})
		errChan <- err
		return err
	}
	return <-errChan
}

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
			goobs.WithEventSubscriptions(subscriptions.InputVolumeMeters|subscriptions.Scenes),
		)
		if err != nil {
			col.Println("Couldn't connect to OBS:", err)
			continue
		}

		sceneResp, err := obs.Scenes.GetCurrentProgramScene()
		if err != nil {
			col.Println("Couldn't get current scene:", err)
			continue
		}
		col.Println("Initial scene:", sceneResp.CurrentProgramSceneName)
		OBSScene.Store(&sceneResp.CurrentProgramSceneName)

		backoff.Success()

		lastMicActivity := time.Now()
		connected := true
		for connected {
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
					connected = false
					break
				}
				switch t := obsEvent.(type) {
				case *events.InputVolumeMeters:
					for _, input := range t.Inputs {
						if input.Name != "Mic" {
							continue
						}
						if len(input.Levels) == 0 {
							continue
						}
						magnitude := 0.0
						for _, levels := range input.Levels {
							magnitude = max(magnitude, levels[0])
						}
						magnitudeDb := 20 * math.Log10(magnitude)
						if magnitudeDb > -35 {
							lastMicActivity = time.Now()
							MicIsSilent.Store(false)
						} else if time.Since(lastMicActivity) > 3*time.Second {
							MicIsSilent.Store(true)
						}
					}
				case *events.CurrentProgramSceneChanged:
					col.Println("Scene changed to", t.SceneName)
					OBSScene.Store(&t.SceneName)
				default:
					col.Printf("Unknown OBS event: %#v\n", t)
				}
			}
		}
	}

}
