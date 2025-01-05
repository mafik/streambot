package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path"
	"time"
)

func PostBluesky(text string) {
	go func() {
		err := PostBlueskySynch(text)
		if err != nil {
			fmt.Println("Error posting to Bluesky:", err)
		}
	}()
}

func PostBlueskySynch(text string) error {
	login, err := ReadStringFromFile(path.Join(baseDir, "secrets", "bsky_login.txt"))
	if err != nil {
		return fmt.Errorf("couldn't read bsky_login.txt: %w", err)
	}
	password, err := ReadStringFromFile(path.Join(baseDir, "secrets", "bsky_password.txt"))
	if err != nil {
		return fmt.Errorf("couldn't read bsky_password.txt: %w", err)
	}

	// First, create a session
	sessionReq := struct {
		Identifier string `json:"identifier"`
		Password   string `json:"password"`
	}{
		Identifier: login,
		Password:   password,
	}

	sessionJSON, err := json.Marshal(sessionReq)
	if err != nil {
		return fmt.Errorf("couldn't marshal session request: %w", err)
	}

	resp, err := http.Post("https://bsky.social/xrpc/com.atproto.server.createSession",
		"application/json",
		bytes.NewBuffer(sessionJSON))
	if err != nil {
		return fmt.Errorf("session request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("session creation failed with status %d: %s", resp.StatusCode, string(body))
	}

	var session struct {
		AccessJwt  string `json:"accessJwt"`
		Did        string `json:"did"`
		Handle     string `json:"handle"`
		RefreshJwt string `json:"refreshJwt"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&session); err != nil {
		return fmt.Errorf("couldn't decode session response: %w", err)
	}

	// Now create the post
	postReq := struct {
		Collection string `json:"collection"`
		Repo       string `json:"repo"`
		Record     struct {
			Text      string `json:"text"`
			CreatedAt string `json:"createdAt"`
			Type      string `json:"$type"`
		} `json:"record"`
	}{
		Collection: "app.bsky.feed.post",
		Repo:       session.Did,
		Record: struct {
			Text      string `json:"text"`
			CreatedAt string `json:"createdAt"`
			Type      string `json:"$type"`
		}{
			Text:      text,
			CreatedAt: time.Now().UTC().Format(time.RFC3339),
			Type:      "app.bsky.feed.post",
		},
	}

	postJSON, err := json.Marshal(postReq)
	if err != nil {
		return fmt.Errorf("couldn't marshal post request: %w", err)
	}

	req, err := http.NewRequest("POST", "https://bsky.social/xrpc/com.atproto.repo.createRecord",
		bytes.NewBuffer(postJSON))
	if err != nil {
		return fmt.Errorf("couldn't create post request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+session.AccessJwt)

	client := &http.Client{}
	resp, err = client.Do(req)
	if err != nil {
		return fmt.Errorf("post request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("post creation failed with status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}
