package main

import (
	"errors"
	"streambot/backoff"

	"github.com/fatih/color"
	"golang.org/x/net/context"
	"golang.org/x/oauth2"
	"google.golang.org/api/option"
	"google.golang.org/api/youtube/v3"
)

const (
	YT_CHANNEL_ID = "UCBPKTkmfqWCVnrEv8CBPrbg"
	YOUTUBE_ICON  = `<img src="youtube.svg" class="emoji">`
)

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

func YouTubeBot() {
	go YouTubeChatBotGRPC()
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
