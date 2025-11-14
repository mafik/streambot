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
	"github.com/andreykaipov/goobs/api/requests/ui"
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

// Returns -1 if not found
func FindMonitorIndex(obs *goobs.Client, displayName string) (int, error) {
	monitorList, err := obs.Ui.GetMonitorList()
	if err != nil {
		return -1, err
	}

	// Find the monitor index by prefix match
	// Note: OBS returns monitor names like "\\.\DISPLAY1(1)" or "VDD by MTT(2)"
	for i, monitor := range monitorList.Monitors {
		if len(monitor.MonitorName) >= len(displayName) &&
			monitor.MonitorName[:len(displayName)] == displayName {
			return i, nil
		}
	}

	return -1, nil
}

func OpenSceneProjector(obs *goobs.Client, sceneName string, monitorIndex int) error {
	_, err := obs.Ui.OpenSourceProjector(&ui.OpenSourceProjectorParams{
		SourceName:   &sceneName,
		MonitorIndex: &monitorIndex,
	})
	return err
}

func ShowAvailableMonitors(obs *goobs.Client) {
	col := color.New(color.FgYellow)

	monitorList, err := obs.Ui.GetMonitorList()
	if err != nil {
		col.Println("Couldn't get monitor list:", err)
		return
	}

	col.Println("Available monitors:")
	for i, monitor := range monitorList.Monitors {
		col.Printf("  [%d] %s (Width: %d, Height: %d)\n", i, monitor.MonitorName, monitor.MonitorWidth, monitor.MonitorHeight)
	}
}

func OpenPreview(obs *goobs.Client, sceneName string, monitorNames []string) error {
	col := color.New(color.FgYellow)

	monitorIndex := -1
	monitorName := ""
	for _, name := range monitorNames {
		idx, err := FindMonitorIndex(obs, name)
		if err != nil {
			col.Println("Error finding monitor index for", name, ":", err)
			return err
		}

		if idx >= 0 {
			monitorName = name
			monitorIndex = idx
			break
		}
	}

	if monitorIndex < 0 {
		col.Println("Could not find display ", monitorNames[0])
		ShowAvailableMonitors(obs)
		return nil
	}

	err := OpenSceneProjector(obs, sceneName, monitorIndex)
	if err != nil {
		col.Println("Couldn't open scene projector for", sceneName, ":", err)
		return err
	}

	col.Println("Opened scene projector:", sceneName, "on", monitorName, "(index", monitorIndex, ")")
	return nil
}

func OBS() {

	col := color.New(color.FgYellow)
	backoff := backoff.Backoff{
		Color:       col,
		Description: "OBS",
	}
	openPreviews := false
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
				openPreviews = true
				col.Println("Starting OBS...")
				// wait up to 30 seconds for OBS to start
				for i := 0; i < 30; i++ {
					probe, err := goobs.New("localhost:4455", goobs.WithPassword(obsPassword))
					if err != nil {
						time.Sleep(time.Second)
						continue
					}
					probe.Disconnect()
					break
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

		if openPreviews {
			openPreviews = false
			OpenPreview(obs, "VDD Mirror", []string{`\\.\DISPLAY1`, `MPI7002`})
			OpenPreview(obs, "Camera Clean", []string{"VDD by MTT"})
		}

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
						if input.Name != "Mic/Aux" {
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
