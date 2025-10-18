package main

import (
	"context"
	"fmt"
	"html"
	"io"
	"streambot/backoff"
	"strings"
	"time"

	"golang.org/x/oauth2"
	"google.golang.org/api/youtube/v3"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
)

func parseISO8601(timestamp string) time.Time {
	t, _ := time.Parse(time.RFC3339, timestamp)
	return t
}

func ptrToString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func YouTubeChatBotGRPC() {
	outerBackoff := backoff.Backoff{
		Color:       youtubeColor,
		Description: "YouTube Live Chat gRPC",
	}

	var pageToken *string = nil
	var accessToken string

	for {
		outerBackoff.Attempt()

		// Find the live broadcast
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
			time.Sleep(15 * time.Second)
			continue
		}

		youtubeVideoId = youtubeBroadcast.Id
		youtubeLiveChatID := youtubeBroadcast.Snippet.LiveChatId

		youtubeColor.Println("Connecting to https://youtu.be/" + youtubeVideoId)

		// Get OAuth token (only if we don't have one or it was cleared due to auth error)
		if accessToken == "" {
			tokenChan := make(chan string)
			YouTubeBotChannel <- func(yt *youtube.Service) error {
				client := getClient(youtube.YoutubeScope)
				token, err := client.Transport.(*oauth2.Transport).Source.Token()
				if err != nil {
					tokenChan <- ""
					return err
				}
				tokenChan <- token.AccessToken
				return nil
			}
			accessToken = <-tokenChan
			if accessToken == "" {
				youtubeColor.Println("Failed to get access token")
				continue
			}
		}

		// Set up gRPC connection
		creds := credentials.NewClientTLSFromCert(nil, "")
		conn, err := grpc.Dial("youtube.googleapis.com:443",
			grpc.WithTransportCredentials(creds),
		)
		if err != nil {
			youtubeColor.Println("Failed to connect to gRPC:", err)
			continue
		}

		grpcClient := NewV3DataLiveChatMessageServiceClient(conn)

		ctx := context.Background()
		ctx = metadata.AppendToOutgoingContext(ctx,
			"authorization", "Bearer "+accessToken,
			"x-goog-user-project", "mafbot-416613",
		)

		outerBackoff.Success()
		firstMessage := true

		innerBackoff := backoff.Backoff{
			Color:       youtubeColor,
			Description: "YouTube gRPC Stream",
		}

		var stream grpc.ServerStreamingClient[LiveChatMessageListResponse]

		for {
			innerBackoff.Attempt()

			if stream == nil {
				maxResults := uint32(2000)
				req := &LiveChatMessageListRequest{
					LiveChatId: &youtubeLiveChatID,
					Part:       []string{"id", "snippet", "authorDetails"},
					MaxResults: &maxResults,
					PageToken:  pageToken,
				}

				Webserver.Call("Ping", "YouTube")
				stream, err = grpcClient.StreamList(ctx, req)
				if err != nil {
					youtubeColor.Printf("Failed to stream chat messages: %v (type: %T)\n", err, err)
					conn.Close()
					// Clear access token on auth failure to force refresh
					if strings.Contains(err.Error(), "Unauthenticated") || strings.Contains(err.Error(), "PermissionDenied") {
						youtubeColor.Println("Auth error detected, will refresh access token on next attempt")
						accessToken = ""
					}
					break
				}
				Webserver.Call("Pong", "YouTube")
			}

			resp, err := stream.Recv()
			Webserver.Call("Ping", "YouTube")
			if err == io.EOF {
				stream = nil
				Webserver.Call("Pong", "YouTube")
				innerBackoff.Success()
				continue
			}
			if err != nil {
				youtubeColor.Printf("Failed to receive chat messages: %v\n", err)
				break
			}
			Webserver.Call("Pong", "YouTube")
			innerBackoff.Success()

			if resp.NextPageToken != nil {
				pageToken = resp.NextPageToken
			}

			if firstMessage { // First requset has the old messages - we don't want to print them
				firstMessage = false
				continue
			}

			for _, item := range resp.Items {
				if item.Snippet == nil || item.Snippet.Type == nil {
					continue
				}
				if *item.Snippet.Type != LiveChatMessageSnippet_TypeWrapper_TEXT_MESSAGE_EVENT {
					continue
				}

				// Get text message details from oneof
				textDetails, ok := item.Snippet.DisplayedContent.(*LiveChatMessageSnippet_TextMessageDetails)
				if !ok || textDetails.TextMessageDetails == nil {
					continue
				}

				chatMessage := ChatEntry{
					Author: User{
						YouTubeUser: &YouTubeUser{
							ChannelID: ptrToString(item.AuthorDetails.ChannelId),
							Name:      ptrToString(item.AuthorDetails.DisplayName),
							AvatarURL: ptrToString(item.AuthorDetails.ProfileImageUrl),
						},
					},
					YouTubeMessageID: ptrToString(item.Id),
					timestamp:        parseISO8601(ptrToString(item.Snippet.PublishedAt)),
				}

				chatMessage.OriginalMessage = ptrToString(textDetails.TextMessageDetails.MessageText)
				chatMessage.HTML = html.EscapeString(chatMessage.OriginalMessage)
				chatMessage.textOnly = chatMessage.OriginalMessage

				// Apply emoji replacements if we have them from InnerTube
				for shortcut, emojiHtml := range ytEmojiShortcutToHTML {
					chatMessage.HTML = strings.ReplaceAll(chatMessage.HTML, shortcut, emojiHtml)
					chatMessage.textOnly = strings.ReplaceAll(chatMessage.textOnly, shortcut, "")
				}

				chatMessage.HTML = YOUTUBE_ICON + " " + chatMessage.Author.HTML() + ": " + chatMessage.HTML
				chatMessage.terminalMsg = fmt.Sprintf("  %s: %s\n", chatMessage.Author.DisplayName(), chatMessage.OriginalMessage)
				chatMessage.ttsMsg = chatMessage.textOnly

				MainChannel <- chatMessage
			}
		}

		conn.Close()
	}
}
