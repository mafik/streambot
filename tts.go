package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"regexp"
	"streambot/backoff"
	"strings"
	"time"

	"github.com/fatih/color"
)

const allTalkUrl = "http://10.0.0.8:7851"
const narratorVoiceCfg = "bg3_narrator.wav"
const defaultVoiceCfg = "SMOrc.wav"

type GenerateResponse struct {
	Status         string `json:"status"`
	OutputFilePath string `json:"output_file_path"`
	OutputFileUrl  string `json:"output_file_url"`
	OutputCacheUrl string `json:"output_cache_url"`
}

type VoicesResponse struct {
	Status string   `json:"status"`
	Voices []string `json:"voices"`
}

func ttsApiRequest(method string, params map[string]string, httpMethod string) *http.Request {
	data := url.Values{}
	for key, value := range params {
		data.Set(key, value)
	}
	r, _ := http.NewRequest(httpMethod, fmt.Sprintf(allTalkUrl+"/api/%s", method), strings.NewReader(data.Encode()))
	r.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	return r
}

func ttsGenerateRequest(text, characterVoice, narratorVoiceArg string) *http.Request {
	params := map[string]string{
		"text_input":          text,
		"text_filtering":      "html",
		"character_voice_gen": characterVoice,
		"language":            "en",
		"output_file_name":    "tts_output",
		"autoplay":            "false",
		"text_not_inside":     "character",
		// "autoplay_volume":     "0.8",
		"temperature": "1.0",
	}
	if narratorVoiceArg != "" {
		params["narrator_enabled"] = "true"
		params["narrator_voice_gen"] = narratorVoiceArg
		params["text_not_inside"] = "character"
	}
	return ttsApiRequest("tts-generate", params, http.MethodPost)
}

func helloWorldRequest() *http.Request {
	return ttsApiRequest("ready", nil, http.MethodPost)
}

var ttsColor = color.New(color.FgBlue)

var TTSChannel = make(chan interface{}, 10)

var htmlTagRegexp = regexp.MustCompile(`<[^>]*>`)
var urlRegexp = regexp.MustCompile(`(https?://[^\s]+)`)
var acronyms = []string{"url", "gpt", "tts", "dns", "http", "ftp"}
var voices []string

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
	return message + " ." // adding dot makes TTS pronounce some short phrases such as "hi"
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

func InitVoices() error {
	voicesReq := ttsApiRequest("voices", nil, http.MethodGet)
	client := &http.Client{}
	voicesResp, err := client.Do(voicesReq)
	if err != nil {
		return fmt.Errorf("couldn't get voices from AllTalk: %w", err)
	}
	body, err := io.ReadAll(voicesResp.Body)
	if err != nil {
		return fmt.Errorf("couldn't read voices response: %w", err)
	}
	var voicesResponse VoicesResponse
	if err := json.Unmarshal(body, &voicesResponse); err != nil {
		return fmt.Errorf("couldn't decode voices response: %w", err)
	}
	voicesDir := path.Join(baseDir, "static", "voices")
	err = os.MkdirAll(voicesDir, 0755)
	if err != nil {
		return fmt.Errorf("couldn't create voices directory: %w", err)
	}
	for _, voice := range voicesResponse.Voices {
		samplePath := path.Join(voicesDir, voice)
		// generate if not exists
		_, err := os.Stat(samplePath)
		if os.IsNotExist(err) {
			fmt.Println("Generating sample for voice:", voice)
			voiceWithoutExtension := strings.TrimSuffix(voice, ".wav")
			sampleReq := ttsGenerateRequest("This is a voice sample in the style of "+voiceWithoutExtension, voice, narratorVoiceCfg)
			wav, err := SynthesizeAllTalk(sampleReq)
			if err != nil {
				return fmt.Errorf("couldn't generate voice sample for %s: %w", voice, err)
			}
			err = os.WriteFile(samplePath, wav, 0644)
			if err != nil {
				return fmt.Errorf("couldn't save voice sample for %s: %w", voice, err)
			}
		}
	}
	voices = voicesResponse.Voices
	return nil
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

			err = InitVoices()
			if err != nil {
				ttsColor.Println("Error while initializing voices:", err)
				continue
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

					userVoice := defaultVoiceCfg
					if t.Author.TwitchUser != nil {
						if userConfig, found := TwitchIndex[t.Author.TwitchUser.Key()]; found {
							userVoice = userConfig.Voice
						}
					}
					if t.Author.YouTubeUser != nil {
						if userConfig, found := YouTubeIndex[t.Author.YouTubeUser.Key()]; found {
							userVoice = userConfig.Voice
						}
					}

					message := VocalizeHTML(t.ttsMsg)
					var r *http.Request
					authorKey := t.Author.Key()
					if lastAuthor == authorKey {
						r = ttsGenerateRequest(fmt.Sprintf("\"%s\"", message), userVoice, narratorVoiceCfg)
					} else {
						r = ttsGenerateRequest(fmt.Sprintf("* %s says: * \" %s \"", t.Author.DisplayName(), message), userVoice, narratorVoiceCfg)
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
					r := ttsGenerateRequest(fmt.Sprintf("* %s *", message), defaultVoiceCfg, narratorVoiceCfg)
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
