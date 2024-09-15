package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"slices"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/gorilla/websocket"
	"github.com/nicklaw5/helix/v2"
	"google.golang.org/api/youtube/v3"
)

const (
	// Time allowed to write a message to the peer.
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer.
	pongWait = 60 * time.Second

	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10

	webserverPort = 3447
)

type WebsocketHub struct {
	// Registered clients.
	clients map[*WebsocketClient]bool

	// Register requests from the clients.
	register chan *WebsocketClient

	// Unregister requests from clients.
	unregister chan *WebsocketClient

	broadcast chan []byte
}

type WebsocketClient struct {
	hub   *WebsocketHub
	conn  *websocket.Conn
	send  chan []byte
	admin bool
	user  *User
}

type callRequest struct {
	Call string        `json:"call"`
	Args []interface{} `json:"args"`
}

func jsonCallRequest(function_name string, args ...interface{}) []byte {
	request := callRequest{
		Call: function_name,
		Args: args,
	}
	jsonRequest, err := json.Marshal(request)
	if err != nil {
		fmt.Println(err)
		return nil
	}
	return jsonRequest
}

func (c *WebsocketClient) Call(function_name string, args ...interface{}) {
	c.send <- jsonCallRequest(function_name, args...)
}

func (c *WebsocketHub) Call(function_name string, args ...interface{}) {
	c.broadcast <- jsonCallRequest(function_name, args...)
}

// writePump pumps messages from the hub to the websocket connection.
//
// A goroutine running writePump is started for each connection. The
// application ensures that there is at most one writer to a connection by
// executing all writes from this goroutine.
func (c *WebsocketClient) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()
	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// The hub closed the channel.
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			err := c.conn.WriteMessage(websocket.TextMessage, message)
			if err != nil {
				fmt.Println(err)
				return
			}
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

type JavaScriptMessage struct {
	Call string            `json:"call"`
	Args []json.RawMessage `json:"args"`
}

type JavaScriptHandler func(*WebsocketClient, ...json.RawMessage)

var JavaScriptHandlers = map[string]JavaScriptHandler{
	"ToggleMuted": ToggleMuted,
	"Ban":         Ban,
	"ShowAlert": func(c *WebsocketClient, args ...json.RawMessage) {
		if !c.admin {
			return
		}
		var html string
		err := json.Unmarshal(args[0], &html)
		if err != nil {
			fmt.Println("Can't unmarshal alert: ", err)
			return
		}
		TTSChannel <- Alert{
			HTML: html,
		}
		fmt.Println("Debug Alert:", html)
	},
	"SetTitle": func(c *WebsocketClient, args ...json.RawMessage) {
		if !c.admin {
			return
		}
		var title string
		err := json.Unmarshal(args[0], &title)
		if err != nil {
			fmt.Println("Can't unmarshal title: ", err)
			return
		}
		if title == "" {
			fmt.Println("Cannot set title to empty string")
			return
		}
		fmt.Printf("Changing stream title to \"%s\"\n", title)
		Webserver.Call("SetStreamTitle", title)
		TwitchHelixChannel <- func(client *helix.Client) {
			if title == twitchTitle {
				return
			}
			resp, err := client.EditChannelInformation(&helix.EditChannelInformationParams{
				BroadcasterID: twitchBroadcasterID,
				Title:         title,
			})
			if err != nil {
				fmt.Println("Couldn't edit Twitch title:", err)
				return
			}
			if resp.StatusCode != 204 {
				fmt.Println("Couldn't edit Twitch title:", resp.ErrorMessage)
				return
			}
			twitchTitle = title
		}
		YouTubeBotChannel <- func(youtube *youtube.Service) error {
			if youtubeVideoId == "" {
				return nil
			}
			listCall := youtube.Videos.List([]string{"id", "snippet"})
			listCall.Id(youtubeVideoId)
			resp, err := listCall.Do()
			if err != nil {
				return err
			}
			if len(resp.Items) == 0 {
				fmt.Println("Couldn't find YouTube video with ID:", youtubeVideoId)
				return nil
			}
			video := resp.Items[0]
			video.Id = youtubeVideoId // this is not returned because we specify fields
			// Note that we need to have the video.Snippet.CategoryId set. That's the entire reason for the first request
			video.Snippet.Title = title
			video.Snippet.Tags = youtubeTags // for some reason the tags are not returned in the first request

			updateCall := youtube.Videos.Update([]string{"id", "snippet"}, video)
			_, err = updateCall.Do()
			if err != nil {
				return err
			}
			return nil
		}

		// Send Twitter notification
		youtubeVideoIdLocal := GetYouTubeVideoID()
		tweet := fmt.Sprintf("🔴 #Automat #LiveCoding: \"%s\"! 🎉🎉🎉\n\n📺 https://youtu.be/%s https://twitch.tv/maf_pl https://tv.algora.io/maf", title, youtubeVideoIdLocal)
		PostTweet(tweet)

	},
	"Password": func(c *WebsocketClient, args ...json.RawMessage) {
		if len(args) != 1 {
			return // don't log anything - could be malicious
		}
		if c.user != nil {
			return // already logged in
		}
		var password string
		err := json.Unmarshal(args[0], &password)
		if err != nil {
			fmt.Println("Can't unmarshal password: ", err)
			return
		}
		c.user = PasswordIndex[password]
		if c.user == nil {
			c.user = &User{}
			PasswordIndex[password] = c.user
		}
		c.user.EnsureTicket()
		c.user.websockets = append(c.user.websockets, c)
		c.Call("Welcome", c.user)
	},
	"ListVoices": func(c *WebsocketClient, args ...json.RawMessage) {
		voicesChan := make(chan []string)
		TTSChannel <- func() {
			voicesChan <- voices
		}
		localVoices := <-voicesChan
		c.Call("ListVoicesResponse", localVoices)
	},
	"SetVoice": func(c *WebsocketClient, args ...json.RawMessage) {
		if c.user == nil {
			return
		}
		var requestedVoice string
		err := json.Unmarshal(args[0], &requestedVoice)
		if err != nil {
			return
		}
		voicesChan := make(chan []string)
		TTSChannel <- func() {
			voicesChan <- voices
		}
		localVoices := <-voicesChan
		found := false
		for _, voice := range localVoices {
			if voice == requestedVoice {
				found = true
				break
			}
		}
		if !found {
			return
		}
		c.user.Voice = requestedVoice
		err = SaveUsers()
		if err != nil {
			fmt.Println("Couldn't save users:", err)
		}
	},
}

// readPump pumps messages from the websocket connection to the hub.
//
// The application runs readPump in a per-connection goroutine. The application
// ensures that there is at most one reader on a connection by executing all
// reads from this goroutine.
func (c *WebsocketClient) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()
	c.conn.SetReadLimit(512)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error { c.conn.SetReadDeadline(time.Now().Add(pongWait)); return nil })
	for {
		_, bytes, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("error: %v", err)
			}
			break
		}
		var message JavaScriptMessage
		err = json.Unmarshal(bytes, &message)
		if err != nil {
			fmt.Println(err)
			continue
		}
		if handler, ok := JavaScriptHandlers[message.Call]; ok {
			handler(c, message.Args...)
		} else {
			fmt.Printf("Unknown JavaScript method: %s(%v)\n", message.Call, message.Args)
		}
	}
}

func OnTwitchWebhook(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("Hello twitch!"))
}

func StartWebserver(OnNewClient chan *WebsocketClient) *WebsocketHub {
	hub := &WebsocketHub{
		register:   make(chan *WebsocketClient),
		unregister: make(chan *WebsocketClient),
		clients:    make(map[*WebsocketClient]bool),
		broadcast:  make(chan []byte),
	}

	upgrader := websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}

	go func() {
		watcher, err := fsnotify.NewWatcher()
		if err != nil {
			fmt.Println("fsnotify.NewWatcher:", err)
			return
		}
		defer watcher.Close()
		err = watcher.Add("static/")
		if err != nil {
			fmt.Println("watcher.Add:", err)
			return
		}
		var reload <-chan time.Time
		for {
			select {
			case <-reload:
				hub.Call("Reload")
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if event.Op.Has(fsnotify.Write) {
					reload = time.After(200 * time.Millisecond)
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				fmt.Println("error:", err)
			}
		}
	}()

	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("./static"))))

	http.HandleFunc("/twitch-auth", OnTwitchAuth)
	http.HandleFunc("/webhook/twitch", OnTwitchWebhook)

	// Turn /live/ into alias for /
	http.Handle("/live/", http.StripPrefix("/live", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.DefaultServeMux.ServeHTTP(w, r)
	})))

	http.HandleFunc("/{$}", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "static/chat.html")
	})

	http.Handle("/", http.FileServer(http.Dir("./static")))

	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Println(err)
			return
		}
		client := &WebsocketClient{hub: hub, conn: conn, send: make(chan []byte, 256)}

		forwarded_headers := r.Header["X-Forwarded-For"]
		switch len(forwarded_headers) {
		case 0:
			// direct connection - local network
			client.admin = IsAuthorized(conn.RemoteAddr().String())
		case 1:
			// nginx proxy
			fmt.Println("Nginx connection from", forwarded_headers[0])
			client.admin = IsAuthorized(forwarded_headers[0])
		default:
			fmt.Println("Hack attempt from", forwarded_headers[len(forwarded_headers)-1], "(multiple X-Forwarded-For headers)")
			// TODO: ban this IP
			client.admin = false
		}

		client.hub.register <- client

		// Allow collection of memory referenced by the caller by doing all work in
		// new goroutines.
		go client.writePump()
		go client.readPump()
	})

	go func() {
		err := http.ListenAndServe(fmt.Sprintf("0.0.0.0:%d", webserverPort), nil)
		if err != nil {
			fmt.Println("ListenAndServe: ", err)
		}
	}()

	go func() {
		for {
			select {
			case message := <-hub.broadcast:
				for client := range hub.clients {
					select {
					case client.send <- message:
					default:
						close(client.send)
						delete(hub.clients, client)
					}
				}
			case client := <-hub.register:
				hub.clients[client] = true

				if OnNewClient != nil {
					select {
					case OnNewClient <- client:
					default:
					}
				}
			case client := <-hub.unregister:
				if _, ok := hub.clients[client]; ok {
					delete(hub.clients, client)
					close(client.send)
				}
				if client.user != nil {
					client.user.websockets = slices.DeleteFunc(client.user.websockets, func(c *WebsocketClient) bool {
						return c == client
					})
					client.user = nil
				}
			}
		}
	}()

	return hub
}
