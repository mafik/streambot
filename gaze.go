package main

import (
	"bufio"
	"fmt"
	"os/exec"
	"path"
	"streambot/backoff"

	"github.com/andreykaipov/goobs"
	"github.com/andreykaipov/goobs/api/requests/sceneitems"
	"github.com/andreykaipov/goobs/api/typedefs"
	"github.com/fatih/color"
)

func gazeExePath() string {
	return path.Join(baseDir, "tobii", "gaze.exe")
}

func ObsGaze(sceneName, sourceName string) {
	col := color.New(color.FgYellow)
	backoff := backoff.Backoff{
		Color:       col,
		Description: "OBS Gaze",
	}
	for {
		backoff.Attempt()

		type State struct {
			sceneItemId             int
			transform               *typedefs.SceneItemTransform
			videoWidth, videoHeight float64
		}
		stateChan := make(chan State)
		OBSChannel <- func(obs *goobs.Client) error {
			var state State
			sceneItemIdResp, err := obs.SceneItems.GetSceneItemId(&sceneitems.GetSceneItemIdParams{
				SceneName:  &sceneName,
				SourceName: &sourceName,
			})
			if err != nil {
				state.sceneItemId = -1
				stateChan <- state
				return fmt.Errorf("couldn't get scene item ID: %w", err)
			}
			state.sceneItemId = sceneItemIdResp.SceneItemId

			getTransformResp, err := obs.SceneItems.GetSceneItemTransform(&sceneitems.GetSceneItemTransformParams{
				SceneName:   &sceneName,
				SceneItemId: &state.sceneItemId,
			})
			if err != nil {
				state.sceneItemId = -1
				stateChan <- state
				return fmt.Errorf("couldn't get scene item transform: %w", err)
			}
			state.transform = getTransformResp.SceneItemTransform
			state.transform.BoundsWidth = state.transform.Width
			state.transform.BoundsHeight = state.transform.Height

			videoSettingsResp, err := obs.Config.GetVideoSettings()
			if err != nil {
				state.sceneItemId = -1
				stateChan <- state
				return fmt.Errorf("couldn't get video settings: %w", err)
			}
			state.videoWidth = videoSettingsResp.BaseWidth
			state.videoHeight = videoSettingsResp.BaseHeight
			stateChan <- state
			return nil
		}
		config := <-stateChan
		if config.sceneItemId == -1 {
			col.Println("Couldn't configure OBS Gaze")
			continue
		}

		// run "gaze.exe" as a separate process with stdout redirected to pipe
		cmd := exec.Command(gazeExePath())
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			col.Printf("Couldn't pipe output of gaze.exe: %s", err)
			continue
		}
		if err := cmd.Start(); err != nil {
			col.Printf("Couldn't start gaze.exe: %s", err)
			continue
		}
		defer cmd.Process.Kill()
		// Read the output of gaze.exe line by line
		scanner := bufio.NewScanner(stdout)

		for scanner.Scan() {
			line := scanner.Text()
			// Parse the line into a GazePoint
			var tobiiX, tobiiY float64
			n, err := fmt.Sscanf(line, "%f %f", &tobiiX, &tobiiY)
			if n != 2 || err != nil {
				col.Printf("Couldn't parse gaze point: %s", line)
				cmd.Process.Kill()
				break
			}

			tobiiX *= 0.98 // duct-tape calibration :P

			alpha := 0.9

			newX := (1+tobiiX)*config.videoWidth/2 - config.transform.Width/2
			newY := (1-tobiiY)*config.videoHeight/2 - config.transform.Height/2

			config.transform.PositionX += (newX - config.transform.PositionX) * alpha
			config.transform.PositionY += (newY - config.transform.PositionY) * alpha
			OBSChannel <- func(obs *goobs.Client) error {
				_, err = obs.SceneItems.SetSceneItemTransform(&sceneitems.SetSceneItemTransformParams{
					SceneName:          &sceneName,
					SceneItemId:        &config.sceneItemId,
					SceneItemTransform: config.transform,
				})
				return err
			}

			backoff.Success()
		}
		col.Printf("gaze.exe exited: %s", cmd.Wait())
	}
}
