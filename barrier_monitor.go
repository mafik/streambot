package main

import (
	"streambot/backoff"
	"strings"
	"time"

	"github.com/fatih/color"
)

type MonitorConfig struct {
	BarrierName string
	OBSScene    string
}

var monitorConfigs = []MonitorConfig{
	{"vr", "Main"},
	{"X1", "NANO"},
	{"WALL-E", "WALL-E"},
}

func BarrierMonitor() {
	col := color.New(color.FgHiGreen)
	sshBackoff := backoff.Backoff{
		Color:       col,
		Description: "Barrier SSH",
	}
	for {
		sshBackoff.Attempt()
		vrSsh, err := NewSSH("vr:17275")
		if err != nil {
			col.Println("Couldn't connect to vr:", err)
			continue
		}
		func() {
			defer vrSsh.Close()
			backoff := backoff.Backoff{
				Color:       col,
				Description: "Barrier Monitor",
			}

			for {
				backoff.Attempt()
				for {
					mouseScreen, err := vrSsh.Exec("tail /run/user/1000/barrier.log -n 10 | rg -NoP 'INFO: switch from \".*\" to \"(.*)\" at' -or '$1' | tail -n 1")
					if err != nil {
						col.Println("Couldn't read barrier log:", err)
						break
					}
					backoff.Success()
					mouseScreen = strings.TrimSpace(mouseScreen)
					// 1. Find the monitor config for this screen
					// 2. Check the current scene in OBS and checck if its one of the scenes in the monitor
					// 3. I
					var obsScene = GetOBSScene()
					obsSceneSwitchable := false
					mouseScreenSwitchable := false
					var targetScene *string = nil
					for _, monitorConfig := range monitorConfigs {
						if monitorConfig.OBSScene == obsScene {
							obsSceneSwitchable = true
						}
						if monitorConfig.BarrierName == mouseScreen {
							mouseScreenSwitchable = true
							targetScene = &monitorConfig.OBSScene
						}
					}

					if obsSceneSwitchable && mouseScreenSwitchable && *targetScene != obsScene {
						col.Println("Switching OBS scene to", *targetScene)
						err = OBSSwitchScene(*targetScene)
						if err != nil {
							col.Println("Couldn't switch OBS scene:", err)
							break
						}
						time.Sleep(5 * time.Second)
					} else {
						time.Sleep(1 * time.Second)
					}
				}

			}
		}() // defer ssh.Close()
	} // for
}
