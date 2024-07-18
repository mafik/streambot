package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"streambot/backoff"
	"strings"
	"time"

	"github.com/fatih/color"
)

const allTalkUrl = "http://10.0.0.8:7851"
const narratorVoiceCfg = "bg3_narrator.wav"

type GenerateResponse struct {
	Status         string `json:"status"`
	OutputFilePath string `json:"output_file_path"`
	OutputFileUrl  string `json:"output_file_url"`
	OutputCacheUrl string `json:"output_cache_url"`
}

func ttsApiRequest(method string, params map[string]string) *http.Request {
	data := url.Values{}
	for key, value := range params {
		data.Set(key, value)
	}
	r, _ := http.NewRequest(http.MethodPost, fmt.Sprintf(allTalkUrl+"/api/%s", method), strings.NewReader(data.Encode()))
	r.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	return r
}

func ttsGenerateRequest(text, characterVoice, narratorVoiceArg string) *http.Request {
	params := map[string]string{
		"text_input":          text,
		"text_filtering":      "standard",
		"character_voice_gen": characterVoice,
		"language":            "en",
		"output_file_name":    "tts_output",
		"autoplay":            "false",
		// "autoplay_volume":     "0.8",
		"temperature": "1.0",
	}
	if narratorVoiceArg != "" {
		params["narrator_enabled"] = "true"
		params["narrator_voice_gen"] = narratorVoiceArg
		params["text_not_inside"] = "character"
	}
	return ttsApiRequest("tts-generate", params)
}

func helloWorldRequest() *http.Request {
	return ttsApiRequest("ready", nil)
}

var ttsColor = color.New(color.FgBlue)

var TTSChannel = make(chan interface{}, 10)

var htmlTagRegexp = regexp.MustCompile(`<[^>]*>`)
var urlRegexp = regexp.MustCompile(`(https?://[^\s]+)`)
var acronyms = []string{"url", "gpt", "tts", "dns", "http", "ftp"}

func Download(url string) ([]byte, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

func TTS() {
	go func() {
		backoff := backoff.Backoff{
			Color:       ttsColor,
			Description: "TTS",
		}
		muted = readMuted()
		var kittyPid string
		defer func() {
			if kittyPid != "" {
				ssh, err := NewSSH("vr:17275")
				if err != nil {
					ttsColor.Println("Couldn't connect to vr to kill AllTalk:", err)
				} else {
					ssh.Exec("kill " + kittyPid)
					ssh.Close()
				}
			}
		}()
		for {
			backoff.Attempt()

			client := &http.Client{}
			r := helloWorldRequest()
			_, err := client.Do(r)
			if err != nil {
				ttsColor.Println("AllTalk is down:", err, ". Starting new instance...")
				ssh, err := NewSSH("vr:17275")
				if err != nil {
					ttsColor.Println("Couldn't connect to vr to start AllTalk:", err)
					continue
				}
				kittyPid, err = ssh.Exec("DISPLAY=:0 kitty /home/maf/Pulpit/Streaming/TTS/alltalk_tts/start_alltalk.sh  >/dev/null 2>&1 & ; echo $last_pid")
				ssh.Close()
				if err != nil {
					ttsColor.Println("Couldn't start AllTalk:", err)
					continue
				}
				kittyPid = strings.TrimSpace(kittyPid)
				// wait up to 60 seconds for AllTalk to start
				operational := false
				for i := 0; i < 60; i++ {
					_, err := client.Do(r)
					if err == nil {
						operational = true
						break
					}
					time.Sleep(time.Second)
				}
				if !operational {
					ttsColor.Println("AllTalk didn't became operational during 60 seconds: ", err)
					continue
				}
			}

			var lastAuthor string
			for msg := range TTSChannel {
				switch t := msg.(type) {
				case ChatEntry:
					client := &http.Client{}

					if muted[t.Author] {
						continue
					}

					message := htmlTagRegexp.ReplaceAllString(t.Message, "")
					message = urlRegexp.ReplaceAllString(message, "\" * U-R-L * \"")

					for _, acronym := range acronyms {
						pronunciation := strings.ToUpper(acronym)
						// insert dashes between letters
						pronunciation = strings.Join(strings.Split(pronunciation, ""), "-")
						message = strings.ReplaceAll(message, acronym, pronunciation)
					}

					var r *http.Request
					if lastAuthor == t.Author {
						r = ttsGenerateRequest(fmt.Sprintf("\"%s\"", message), "SMOrc.wav", narratorVoiceCfg)
					} else {
						r = ttsGenerateRequest(fmt.Sprintf("*%s says: * \"%s\"", t.Author, message), "SMOrc.wav", narratorVoiceCfg)
						lastAuthor = t.Author
					}
					resp, err := client.Do(r)
					if err != nil {
						ttsColor.Println("Couldn't generate TTS:", err)
						break
					}
					if resp.StatusCode != http.StatusOK {
						ttsColor.Println("TTS API returned non-200 status code:", resp.Status)
						break
					}
					body, err := io.ReadAll(resp.Body)
					if err != nil {
						ttsColor.Println("Couldn't read TTS response:", err)
						break
					}
					var generateResponse GenerateResponse
					if err := json.Unmarshal(body, &generateResponse); err != nil {
						ttsColor.Println("Couldn't decode TTS response:", err)
						break
					}

					func() {
						wav, err := Download(allTalkUrl + generateResponse.OutputFileUrl)
						if err != nil {
							ttsColor.Println("Couldn't download TTS result:", err)
							return
						}
						select {
						case AudioPlayerChannel <- wav:
						default:
							ttsColor.Println("Player is busy, dropping TTS message")
						}
					}()
				case func():
					t()
				}
			}
		} // for
	}()
}
