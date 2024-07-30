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

func VocalizeHTML(text string) string {
	message := htmlTagRegexp.ReplaceAllString(text, "")
	message = urlRegexp.ReplaceAllString(message, "\" * U-R-L * \"")
	for _, acronym := range acronyms {
		pronunciation := strings.ToUpper(acronym)
		// insert dashes between letters
		pronunciation = strings.Join(strings.Split(pronunciation, ""), "-")
		message = strings.ReplaceAll(message, acronym, pronunciation)
	}
	return message
}

func SynthesizeAllTalk(r *http.Request) ([]byte, error) {
	client := &http.Client{}
	resp, err := client.Do(r)
	if err != nil {
		return nil, fmt.Errorf("couldn't generate TTS: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("TTS API returned non-200 status code: %s", resp.Status)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("couldn't read TTS response: %w", err)
	}
	var generateResponse GenerateResponse
	if err := json.Unmarshal(body, &generateResponse); err != nil {
		return nil, fmt.Errorf("couldn't decode TTS response: %w", err)
	}
	wav, err := Download(allTalkUrl + generateResponse.OutputFileUrl)
	if err != nil {
		return nil, fmt.Errorf("couldn't download TTS result: %w", err)
	}
	return wav, nil
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
				ttsColor.Println("AllTalk is down. Starting new instance...")
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
					if IsMuted(t.Author) {
						continue
					}
					if t.ttsMsg == "" {
						continue
					}

					message := VocalizeHTML(t.ttsMsg)
					var r *http.Request
					authorKey := t.Author.Key()
					if lastAuthor == authorKey {
						r = ttsGenerateRequest(fmt.Sprintf("\"%s\"", message), "SMOrc.wav", narratorVoiceCfg)
					} else {
						r = ttsGenerateRequest(fmt.Sprintf("*%s says: * \"%s\"", t.Author.DisplayName(), message), "SMOrc.wav", narratorVoiceCfg)
						lastAuthor = authorKey
					}
					wav, err := SynthesizeAllTalk(r)
					if err != nil {
						ttsColor.Println("AllTalk error:", err)
						break
					}
					select {
					case AudioPlayerChannel <- PlayMessage{
						wavData: wav,
						author:  &t.Author,
					}:
					default:
						ttsColor.Println("Player is busy, dropping TTS message")
					}
				case Alert:
					message := VocalizeHTML(t.HTML)
					r := ttsGenerateRequest(fmt.Sprintf("* %s *", message), "SMOrc.wav", narratorVoiceCfg)
					wav, err := SynthesizeAllTalk(r)
					if err != nil {
						ttsColor.Println("AllTalk error:", err)
						break
					}
					select {
					case AudioPlayerChannel <- PlayMessage{
						wavData: wav,
						prePlay: func() {
							durationMillis := WAVDuration(wav).Milliseconds()
							if t.onPlay != nil {
								t.onPlay()
							}
							Webserver.Call("ShowAlert", t.HTML, durationMillis)
							// block audio playback for 1 second (until alert window opens)
							time.Sleep(time.Second)
						},
						postPlay: func() {
							time.Sleep(time.Second)
						},
					}:
					default:
						ttsColor.Println("Player is busy, dropping TTS message")
					}
				case func():
					t()
				}
			}
		} // for
	}()
}
