package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path"
	"strings"

	"github.com/dghubble/oauth1"
)

func PostTweet(tweetText string) {
	go func() {
		err := PostTweetSync(tweetText)
		if err != nil {
			fmt.Println(err)
		}
	}()
}

func PostTweetSync(tweetText string) error {
	type Tweet struct {
		Text string `json:"text"`
	}
	jsonData, err := json.Marshal(Tweet{Text: tweetText})
	if err != nil {
		return fmt.Errorf("couldn't marshal tweet: %w", err)
	}
	apiKey, err := ReadStringFromFile(path.Join(baseDir, "secrets", "twitter_api_key.txt"))
	if err != nil {
		return fmt.Errorf("couldn't read twitter_api_key.txt: %w", err)
	}
	apiKeySecret, err := ReadStringFromFile(path.Join(baseDir, "secrets", "twitter_api_key_secret.txt"))
	if err != nil {
		return fmt.Errorf("couldn't read twitter_api_key_secret.txt: %w", err)
	}
	accessToken, err := ReadStringFromFile(path.Join(baseDir, "secrets", "twitter_access_token.txt"))
	if err != nil {
		return fmt.Errorf("couldn't read twitter_access_token.txt: %w", err)
	}
	accessTokenSecret, err := ReadStringFromFile(path.Join(baseDir, "secrets", "twitter_access_token_secret.txt"))
	if err != nil {
		return fmt.Errorf("couldn't read twitter_access_token_secret.txt: %w", err)
	}
	oauth1Config := oauth1.NewConfig(apiKey, apiKeySecret)
	token := oauth1.NewToken(accessToken, accessTokenSecret)
	req, err := http.NewRequest(http.MethodPost, "https://api.twitter.com/2/tweets", strings.NewReader(string(jsonData)))
	if err != nil {
		return fmt.Errorf("couldn't create a new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	httpClient := oauth1Config.Client(req.Context(), token)
	res, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("couldn't do the request: %w", err)
	}
	if res.StatusCode != http.StatusCreated {
		var tweetError struct {
			Title  string `json:"title"`
			Type   string `json:"type"`
			Detail string `json:"detail"`
			Status int    `json:"status"`
		}
		b, _ := io.ReadAll(res.Body)
		err = json.Unmarshal(b, &tweetError)
		if err != nil {
			return fmt.Errorf("couldn't unmarshal the tweet error: %w the body looks like so %s", err, b)
		}
		return fmt.Errorf("tweet error: Title=[%s] Type=[%s] Detail=[%s] Status=%d", tweetError.Title, tweetError.Type, tweetError.Detail, tweetError.Status)
	}
	defer res.Body.Close()
	var successfulTweetRes struct {
		Data struct {
			ID                  string   `json:"id"`
			Text                string   `json:"text"`
			EditHistoryTweetIDs []string `json:"edit_history_tweet_ids"`
		} `json:"data"`
	}
	b, err := io.ReadAll(res.Body)
	if err != nil {
		return fmt.Errorf("couldn't read the response body: %w", err)
	}
	err = json.Unmarshal(b, &successfulTweetRes)
	if err != nil {
		return fmt.Errorf("failed to unmarshall the successful tweet response, maybe it failed? : %w the body looks like so %s", err, b)
	}
	return nil
}
