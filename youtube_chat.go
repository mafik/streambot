package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io/ioutil"
	"math/rand"
	"net/http"
	"regexp"
	"strconv"
	"streambot/backoff"
	"strings"
	"time"

	"github.com/fatih/color"
	"golang.org/x/net/context"
	"google.golang.org/api/option"
	"google.golang.org/api/youtube/v3"
)

type SubMenuItems struct {
	Title        string
	Continuation struct {
		ReloadContinuationData struct {
			Continuation string
		}
	}
}

type InnerTubeContext struct {
	Client struct {
		Hl               string `json:"hl"`
		Gl               string `json:"gl"`
		RemoteHost       string `json:"remoteHost"`
		DeviceMake       string `json:"deviceMake"`
		DeviceModel      string `json:"deviceModel"`
		VisitorData      string `json:"visitorData"`
		UserAgent        string `json:"userAgent"`
		ClientName       string `json:"clientName"`
		ClientVersion    string `json:"clientVersion"`
		OsName           string `json:"osName"`
		OsVersion        string `json:"osVersion"`
		OriginalUrl      string `json:"originalUrl"`
		Platform         string `json:"platform"`
		ClientFormFactor string `json:"clientFormFactor"`
		ConfigInfo       struct {
			AppInstallData string `json:"appInstallData"`
		} `json:"configInfo"`
	} `json:"client"`
}

type YtCfg struct {
	INNERTUBE_API_KEY             string
	INNERTUBE_CONTEXT             InnerTubeContext
	INNERTUBE_CONTEXT_CLIENT_NAME string
	INNERTUBE_CLIENT_VERSION      string
	ID_TOKEN                      string
}

type Context struct {
	Context      InnerTubeContext `json:"context"`
	Continuation string           `json:"continuation"`
}

type ContinuationChat struct {
	TimedContinuationData struct {
		Continuation string `json:"continuation"`
		TimeoutMs    int    `json:"timeoutMs"`
	} `json:"timedContinuationData"`
	InvalidationContinuationData struct {
		Continuation string `json:"continuation"`
		TimeoutMs    int    `json:"timeoutMs"`
	} `json:"invalidationContinuationData"`
}
type Actions struct {
	AddChatItemAction struct {
		Item struct {
			LiveChatTextMessageRenderer struct {
				Message struct {
					Runs []Runs `json:"runs"`
				} `json:"message"`
				AuthorName struct {
					SimpleText string `json:"simpleText"`
				}
				TimestampUsec string `json:"timestampUsec"`
			} `json:"liveChatTextMessageRenderer"`
		} `json:"item"`
	} `json:"addChatItemAction"`
}

type Runs struct {
	Text  string `json:"text,omitempty"`
	Emoji struct {
		EmojiId       string `json:"emojiId"`
		IsCustomEmoji bool   `json:"isCustomEmoji,omitempty"`
		Image         struct {
			Thumbnails []struct {
				Url string `json:"url,omitempty"`
			}
		}
	} `json:"emoji,omitempty"`
}

type ChatMessagesResponse struct {
	ContinuationContents struct {
		LiveChatContinuation struct {
			Actions       []Actions          `json:"actions"`
			Continuations []ContinuationChat `json:"continuations"`
		} `json:"liveChatContinuation"`
	} `json:"continuationContents"`
}

type ErrorResponse struct {
	Error struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Errors  []struct {
			Message string `json:"message"`
			Domain  string `json:"domain"`
			Reason  string `json:"reason"`
		} `json:"errors"`
		Status string `json:"status"`
	} `json:"error"`
}

type ChatMessage struct {
	AuthorName string
	Message    string
	Timestamp  time.Time
}

type InitialData struct {
	Contents struct {
		TwoColumnWatchNextResults struct {
			ConversationBar struct {
				LiveChatRenderer struct {
					Header struct {
						LiveChatHeaderRenderer struct {
							ViewSelector struct {
								SortFilterSubMenuRenderer struct {
									SubMenuItems []SubMenuItems `json:"subMenuItems"`
								}
							}
						}
					}
				}
			}
		}
	}
}

var (
	LIVE_CHAT_URL = `https://www.youtube.com/youtubei/v1/live_chat/get_%s?key=%s`
	// Google would sometimes ask you to solve a CAPTCHA before accessing it's websites
	// or ask for your CONSENT if you are an EU user
	// You can add those cookies here.
	customCookies     []*http.Cookie
	ErrLiveStreamOver error = errors.New("live stream over")
	ErrStreamNotLive  error = errors.New("stream not live")
)

const (
	API_TYPE           = "live_chat"
	YT_CFG_REGEX       = `ytcfg\.set\s*\(\s*({.+?})\s*\)\s*;`
	INITIAL_DATA_REGEX = `(?:window\s*\[\s*["\']ytInitialData["\']\s*\]|ytInitialData)\s*=\s*({.+?})\s*;\s*(?:var\s+meta|</script|\n)`
)

func regexSearch(regex string, str string) []string {
	r, _ := regexp.Compile(regex)
	matches := r.FindAllString(str, -1)
	return matches
}

func parseMicroSeconds(timeStampStr string) time.Time {
	tm, _ := strconv.ParseInt(timeStampStr, 10, 64)
	tm = tm / 1000
	sec := tm / 1000
	msec := tm % 1000
	return time.Unix(sec, msec*int64(time.Millisecond))
}

func FetchChatMessages(initialContinuationInfo string, ytCfg YtCfg) ([]ChatMessage, string, int, error) {
	apiKey := ytCfg.INNERTUBE_API_KEY
	continuationUrl := fmt.Sprintf(LIVE_CHAT_URL, API_TYPE, apiKey)
	innertubeContext := ytCfg.INNERTUBE_CONTEXT

	context := Context{innertubeContext, initialContinuationInfo}
	b, _ := json.Marshal(context)
	var jsonData = []byte(b)
	request, error := http.NewRequest("POST", continuationUrl, bytes.NewBuffer(jsonData))
	if error != nil {
		return nil, "", 0, error
	}
	request.Header.Set("Content-Type", "application/json; charset=UTF-8")

	client := &http.Client{}
	response, error := client.Do(request)
	if error != nil {
		return nil, "", 0, error
	}
	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, "", 0, err
	}

	if response.StatusCode != 200 {
		var errResp ErrorResponse
		json.Unmarshal(body, &errResp)
		return nil, "", 0, fmt.Errorf("got status code %d (%s)", response.StatusCode, errResp.Error.Message)
	}

	response.Body.Close()
	var chatMsgResp ChatMessagesResponse
	json.Unmarshal([]byte(string(body)), &chatMsgResp)
	actions := chatMsgResp.ContinuationContents.LiveChatContinuation.Actions

	chatMessages := []ChatMessage{}
	for _, action := range actions {
		liveChatTextMessageRenderer := action.AddChatItemAction.Item.LiveChatTextMessageRenderer
		// Each chat message is seperated into multiple runs.
		// Iterate through all runs and generate the chat message.
		runs := liveChatTextMessageRenderer.Message.Runs
		if len(runs) > 0 {
			chatMessage := ChatMessage{}
			authorName := liveChatTextMessageRenderer.AuthorName.SimpleText
			chatMessage.Timestamp = parseMicroSeconds(liveChatTextMessageRenderer.TimestampUsec)
			chatMessage.AuthorName = authorName
			text := ""
			for _, run := range runs {
				if run.Text != "" {
					text += run.Text
				} else {
					if run.Emoji.IsCustomEmoji {
						numberOfThumbnails := len(run.Emoji.Image.Thumbnails)
						// Youtube chat has custom emojis which
						// are small PNG images and cannot be displayed as text.
						//
						// These custom emojis are available with their image url.
						//
						// Adding some whitespace around custom image URLs
						// without the whitespace it would be difficult to parse these URLs
						if numberOfThumbnails > 0 && numberOfThumbnails == 2 {
							text += " " + run.Emoji.Image.Thumbnails[1].Url + " "
						} else if numberOfThumbnails == 1 {
							text += " " + run.Emoji.Image.Thumbnails[0].Url + " "
						}
					} else {
						text += run.Emoji.EmojiId
					}
				}
			}
			chatMessage.Message = text
			chatMessages = append(chatMessages, chatMessage)
		}
	}
	// No continuation returned from youtube, Stream has ended.
	if len(chatMsgResp.ContinuationContents.LiveChatContinuation.Continuations) == 0 {
		return nil, "", 0, ErrLiveStreamOver
	}
	// extract continuation and timeout received from response
	timeoutMs := 5
	continuations := chatMsgResp.ContinuationContents.LiveChatContinuation.Continuations[0]
	if continuations.TimedContinuationData.Continuation == "" {
		initialContinuationInfo = continuations.InvalidationContinuationData.Continuation
		timeoutMs = continuations.InvalidationContinuationData.TimeoutMs
	} else {
		initialContinuationInfo = continuations.TimedContinuationData.Continuation
		timeoutMs = continuations.TimedContinuationData.TimeoutMs
	}
	return chatMessages, initialContinuationInfo, timeoutMs, nil
}

func ParseInitialData(videoUrl string) (string, YtCfg, error) {
	req, err := http.NewRequest("GET", videoUrl, nil)
	if err != nil {
		return "", YtCfg{}, err
	}

	for _, cookie := range customCookies {
		req.AddCookie(cookie)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", YtCfg{}, err
	}

	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		fmt.Print(videoUrl +
			"\nresp.StatusCode: " + strconv.Itoa(resp.StatusCode))
		return "", YtCfg{}, err
	}

	intArr, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", YtCfg{}, err
	}

	html := string(intArr)

	// TODO ::  work on regex
	initialDataArr := regexSearch(INITIAL_DATA_REGEX, html)
	initialData := strings.TrimPrefix(string(initialDataArr[0]), "ytInitialData = ")
	initialData = strings.TrimSuffix(initialData, ";</script")
	ytCfg := regexSearch(YT_CFG_REGEX, html)[0]
	ytCfg = strings.TrimPrefix(ytCfg, "ytcfg.set(")
	ytCfg = strings.TrimSuffix(ytCfg, ");")

	var _ytCfg YtCfg
	json.Unmarshal([]byte(ytCfg), &_ytCfg)

	var _initialData InitialData
	json.Unmarshal([]byte(initialData), &_initialData)

	subMenuItems := _initialData.Contents.TwoColumnWatchNextResults.ConversationBar.LiveChatRenderer.Header.LiveChatHeaderRenderer.ViewSelector.SortFilterSubMenuRenderer.SubMenuItems
	if len(subMenuItems) == 0 {
		return "", YtCfg{}, ErrStreamNotLive
	}
	initialContinuationInfo := subMenuItems[1].Continuation.ReloadContinuationData.Continuation
	return initialContinuationInfo, _ytCfg, nil
}

func AddCookies(cookies []*http.Cookie) {
	customCookies = cookies
}

var youtubeEmojiRegexp = regexp.MustCompile(`(https://yt3.ggpht.com/[^ ]+)`)

var YouTubeBotChannel = make(chan interface{})

func YouTubeBot() {
	color := color.New(color.FgRed)
	outerBackoff := backoff.Backoff{
		Color:       color,
		Description: "YouTube Live Chat",
	}
	for {
		outerBackoff.Attempt()

		client := getClient(false, youtube.YoutubeScope)
		youtube, err := youtube.NewService(context.Background(), option.WithHTTPClient(client))
		if err != nil {
			color.Println("Error creating YouTube client:", err)
			continue
		}
		call := youtube.LiveBroadcasts.List([]string{"id", "snippet", "status"})
		call.Mine(true)
		listResp, err := call.Do()
		if err != nil {
			color.Println("Error in search:", err)
			continue
		}

		var videoId string
		for _, result := range listResp.Items {
			if result.Status.LifeCycleStatus == "complete" {
				continue
			}
			// result.Snippet.LiveChatId
			videoId = result.Id
		}
		if videoId == "" {
			// TODO: create one ourselves...
			color.Println("No live stream found. Visit YouTube live streaming dashboard to create a new one!")
			continue
		}
		color.Println("Connecting to https://youtu.be/" + videoId)

		customCookies := []*http.Cookie{
			{Name: "PREF",
				Value:  "tz=Europe.Rome",
				MaxAge: 300},
			{Name: "CONSENT",
				Value:  fmt.Sprintf("YES+yt.432048971.it+FX+%d", 100+rand.Intn(999-100+1)),
				MaxAge: 300},
		}
		AddCookies(customCookies)
		continuation, cfg, error := ParseInitialData("https://www.youtube.com/watch?v=" + videoId)
		if error != nil {
			color.Println("Error in ParseInitialData:", error)
			continue
		}
		outerBackoff.Success()
		firstRequest := true
		innerBackoff := backoff.Backoff{
			Color:       color,
			Description: "YouTube Live Chat",
		}
		for {
			innerBackoff.Attempt()
			chat, newContinuation, sleepMillis, error := FetchChatMessages(continuation, cfg)
			if error == ErrLiveStreamOver {
				color.Println("Live stream over")
				break
			}
			if error != nil {
				color.Println("Error in FetchChatMessages", error)
				continue
			}
			innerBackoff.Success()
			continuation = newContinuation

			if firstRequest {
				// First requset has the old messages - we don't want to print them
				firstRequest = false
				continue
			}

			for _, msg := range chat {
				entry := ChatEntry{
					Author:  msg.AuthorName,
					Message: msg.Message,
					Source:  "YouTube",
				}

				entry.terminalMsg = fmt.Sprintf("  %s: %s\n", entry.Author, entry.Message)
				entry.Message = html.EscapeString(entry.Message)
				entry.Message = youtubeEmojiRegexp.ReplaceAllString(entry.Message, `<img src="$1" alt="YouTube emoji" class="emoji">`)

				MainChannel <- entry
			}
			if sleepMillis > 1000 {
				sleepMillis = 1000
			}
			time.Sleep(time.Duration(sleepMillis) * time.Millisecond)
		} // inner for
	} // outer for
}
