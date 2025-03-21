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
	"golang.org/x/oauth2"
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
type Thumbnail struct {
	Url    string `json:"url"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
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
				} `json:"authorName"`
				AuthorPhoto struct {
					Thumbnails []Thumbnail
				} `json:"authorPhoto"`
				AuthorExternalChannelId string `json:"authorExternalChannelId"`
				TimestampUsec           string `json:"timestampUsec"`
			} `json:"liveChatTextMessageRenderer"`
		} `json:"item"`
	} `json:"addChatItemAction"`
}

type Runs struct {
	Text  string `json:"text,omitempty"`
	Emoji struct {
		EmojiId       string   `json:"emojiId"`
		IsCustomEmoji bool     `json:"isCustomEmoji,omitempty"`
		Shortcuts     []string `json:"shortcuts,omitempty"`
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
	YT_CHANNEL_ID      = "UCBPKTkmfqWCVnrEv8CBPrbg"
	YOUTUBE_ICON       = `<img src="youtube.svg" class="emoji">`
)

var ytEmojiShortcutToHTML = make(map[string]string)

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

func FetchChatMessages(initialContinuationInfo string, ytCfg YtCfg) ([]ChatEntry, string, int, error) {
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

	chatMessages := []ChatEntry{}
	for _, action := range actions {
		liveChatTextMessageRenderer := action.AddChatItemAction.Item.LiveChatTextMessageRenderer
		// Each chat message is seperated into multiple runs.
		// Iterate through all runs and generate the chat message.
		runs := liveChatTextMessageRenderer.Message.Runs
		if len(runs) > 0 {
			chatMessage := ChatEntry{
				Author: User{
					YouTubeUser: &YouTubeUser{
						Name:      liveChatTextMessageRenderer.AuthorName.SimpleText,
						ChannelID: liveChatTextMessageRenderer.AuthorExternalChannelId,
					},
				},
				YouTubeMessageID: "unknown",
				timestamp:        parseMicroSeconds(liveChatTextMessageRenderer.TimestampUsec),
			}
			bestSize := 0
			for _, thumbnail := range liveChatTextMessageRenderer.AuthorPhoto.Thumbnails {
				if thumbnail.Width > bestSize {
					chatMessage.Author.YouTubeUser.AvatarURL = thumbnail.Url
					bestSize = thumbnail.Width
				}
			}

			for _, run := range runs {
				for _, shortcut := range run.Emoji.Shortcuts {
					url := ""
					numberOfThumbnails := len(run.Emoji.Image.Thumbnails)
					if numberOfThumbnails > 0 {
						url = run.Emoji.Image.Thumbnails[numberOfThumbnails-1].Url
					}
					ytEmojiShortcutToHTML[shortcut] = fmt.Sprintf(`<img src="%s" alt="YouTube emoji" class="emoji">`, url)
				}
				if run.Text != "" {
					chatMessage.terminalMsg += run.Text
					chatMessage.HTML += html.EscapeString(run.Text)
					chatMessage.OriginalMessage += run.Text
					chatMessage.textOnly += run.Text
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
						chatMessage.terminalMsg += "[emoji]"
						url := ""
						if numberOfThumbnails > 0 && numberOfThumbnails == 2 {
							url = run.Emoji.Image.Thumbnails[1].Url
						} else if numberOfThumbnails == 1 {
							url = run.Emoji.Image.Thumbnails[0].Url
						}
						chatMessage.HTML += fmt.Sprintf(`<img src="%s" alt="YouTube emoji" class="emoji">`, url)
					} else {
						chatMessage.terminalMsg += run.Emoji.EmojiId
						chatMessage.HTML += run.Emoji.EmojiId
					}
				}
			}

			chatMessage.ttsMsg = chatMessage.HTML
			chatMessage.terminalMsg = fmt.Sprintf("  %s: %s\n", chatMessage.Author.DisplayName(), chatMessage.terminalMsg)
			chatMessage.HTML = fmt.Sprintf(YOUTUBE_ICON+` %s: %s`, chatMessage.Author.HTML(), chatMessage.HTML)

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

type YouTubeFunc func(*youtube.Service) error

var YouTubeBotChannel = make(chan YouTubeFunc)
var youtubeColor = color.New(color.FgRed)

// This should only be accessed from YT goroutine use `GetYouTubeVideoID` instead.
var youtubeVideoId string

func GetYouTubeVideoID() string {
	youtubeVideoIdChan := make(chan string)
	YouTubeBotChannel <- func(youtube *youtube.Service) error {
		youtubeVideoIdChan <- youtubeVideoId
		return nil
	}
	return <-youtubeVideoIdChan
}

func parseISO8601(timestamp string) time.Time {
	t, _ := time.Parse(time.RFC3339, timestamp)
	return t
}

var youtubeLiveChatID string

func YouTubeChatBot() {
	outerBackoff := backoff.Backoff{
		Color:       youtubeColor,
		Description: "YouTube Live Chat",
	}
	for {
		outerBackoff.Attempt()
		videoIdChan := make(chan *youtube.LiveBroadcast)
		YouTubeBotChannel <- func(youtube *youtube.Service) error {
			call := youtube.LiveBroadcasts.List([]string{"id", "snippet", "status"})
			call.Mine(true)
			listResp, err := call.Do()
			if err != nil {
				return err
			}

			for _, result := range listResp.Items {
				if result.Status.LifeCycleStatus == "complete" {
					continue
				}
				videoIdChan <- result
				return nil
			}
			videoIdChan <- nil
			return nil
		}
		youtubeBroadcast := <-videoIdChan

		if youtubeBroadcast == nil {
			dashboardURL := fmt.Sprintf("https://studio.youtube.com/channel/%s/livestreaming/dashboard?c=%s", YT_CHANNEL_ID, YT_CHANNEL_ID)
			youtubeColor.Printf("No live stream found. Opening %s to create a new one!\n", dashboardURL)
			openURL(dashboardURL)
			time.Sleep(15 * time.Second) // give some time for the YT dashboard to create a new stream
			continue
		}
		youtubeVideoId = youtubeBroadcast.Id
		youtubeLiveChatID = youtubeBroadcast.Snippet.LiveChatId

		youtubeColor.Println("Connecting to https://youtu.be/" + youtubeVideoId)

		customCookies := []*http.Cookie{
			{Name: "PREF",
				Value:  "tz=Europe.Rome",
				MaxAge: 300},
			{Name: "CONSENT",
				Value:  fmt.Sprintf("YES+yt.432048971.it+FX+%d", 100+rand.Intn(999-100+1)),
				MaxAge: 300},
		}
		AddCookies(customCookies)
		continuation, cfg, err := ParseInitialData("https://www.youtube.com/watch?v=" + youtubeVideoId)
		if err != nil {
			youtubeColor.Println("Error in ParseInitialData:", err)
			continue
		}
		outerBackoff.Success()

		// Get the initial nextPageToken
		// We ignore the initial messages - they're likely already in the chat log
		nextPageChan := make(chan string)
		YouTubeBotChannel <- func(yt *youtube.Service) error {
			call := yt.LiveChatMessages.List(youtubeLiveChatID, []string{"id", "snippet"})
			call.MaxResults(2000)
			resp, err := call.Do()
			if err != nil {
				nextPageChan <- ""
				return err
			}
			nextPageChan <- resp.NextPageToken
			return nil
		}
		nextPageToken := <-nextPageChan
		if nextPageToken == "" {
			youtubeColor.Println("Error getting initial nextPageToken")
			continue
		}

		firstRequest := true
		innerBackoff := backoff.Backoff{
			Color:       youtubeColor,
			Description: "YouTube Live Chat Loop",
		}
		for {
			innerBackoff.Attempt()
			Webserver.Call("Ping", "YouTube")
			chat, newContinuation, sleepMillis, err := FetchChatMessages(continuation, cfg)
			if err == ErrLiveStreamOver {
				youtubeColor.Println("Live stream over")
				break
			}
			if err != nil {
				youtubeColor.Println("Error in FetchChatMessages", err)
				continue
			}
			innerBackoff.Success()
			Webserver.Call("Pong", "YouTube")
			continuation = newContinuation

			if firstRequest {
				// First requset has the old messages - we don't want to print them
				firstRequest = false
				continue
			}

			if len(chat) > 0 {
				// Hacky YT client detected new chat message - let's try to fetch it using the official API
				YouTubeBotChannel <- func(yt *youtube.Service) error {
					call := yt.LiveChatMessages.List(youtubeLiveChatID, []string{"id", "snippet", "authorDetails"})
					call.PageToken(nextPageToken)
					call.MaxResults(2000)
					resp, err := call.Do()
					if err != nil {
						nextPageChan <- nextPageToken
						return err
					}
					nextPageChan <- resp.NextPageToken
					for _, item := range resp.Items {
						if item.Snippet.Type != "textMessageEvent" {
							continue
						}

						chatMessage := ChatEntry{
							Author: User{
								YouTubeUser: &YouTubeUser{
									ChannelID: item.AuthorDetails.ChannelId,
									Name:      item.AuthorDetails.DisplayName,
									AvatarURL: item.AuthorDetails.ProfileImageUrl,
								},
							},
							YouTubeMessageID: item.Id,
							timestamp:        parseISO8601(item.Snippet.PublishedAt),
						}

						chatMessage.OriginalMessage = item.Snippet.TextMessageDetails.MessageText
						chatMessage.HTML = html.EscapeString(chatMessage.OriginalMessage)
						chatMessage.textOnly = chatMessage.OriginalMessage
						for shortcut, emojiHtml := range ytEmojiShortcutToHTML {
							chatMessage.HTML = strings.ReplaceAll(chatMessage.HTML, shortcut, emojiHtml)
							chatMessage.textOnly = strings.ReplaceAll(chatMessage.textOnly, shortcut, "")
						}
						chatMessage.HTML = YOUTUBE_ICON + " " + chatMessage.Author.HTML() + ": " + chatMessage.HTML
						chatMessage.terminalMsg = fmt.Sprintf("  %s: %s\n", chatMessage.Author.DisplayName(), chatMessage.OriginalMessage)
						chatMessage.ttsMsg = chatMessage.textOnly
						MainChannel <- chatMessage
					}
					return nil
				}
				nextPageToken = <-nextPageChan

			}

			if sleepMillis > 1000 {
				sleepMillis = 1000
			}
			time.Sleep(time.Duration(sleepMillis) * time.Millisecond)
		} // inner for
	} // outer for
}

func YouTubeBot() {
	go YouTubeChatBot()
	backoff := backoff.Backoff{
		Color:       youtubeColor,
		Description: "YouTube API",
	}
	for {
		backoff.Attempt()

		client := getClient(youtube.YoutubeScope)
		youtube, err := youtube.NewService(context.Background(), option.WithHTTPClient(client))
		if err != nil {
			youtubeColor.Println("Error creating YouTube client:", err)
			continue
		}

		for fun := range YouTubeBotChannel {
			err = fun(youtube)
			if err != nil {
				var retrieveError *oauth2.RetrieveError
				if errors.As(err, &retrieveError) {
					if retrieveError.ErrorCode == "invalid_grant" {
						youtubeColor.Println("Invalid grant. Deleting bad token... Authorization page will be open on the next attempt.")
						clearYouTubeToken()
						break
					}
				}
				youtubeColor.Println("Error in YouTube:", err)
				continue
			} else {
				backoff.Success()
			}
		}
	}
}
