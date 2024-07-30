package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/gorilla/websocket"
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
	hub  *WebsocketHub
	conn *websocket.Conn
	send chan []byte
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

type JavaScriptHandler func(...json.RawMessage)

var JavaScriptHandlers = map[string]JavaScriptHandler{
	"ToggleMuted": ToggleMuted,
	"Ban":         Ban,
	"ShowAlert": func(args ...json.RawMessage) {
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
		if IsAuthorized(c.conn.RemoteAddr().String()) {
			if handler, ok := JavaScriptHandlers[message.Call]; ok {
				handler(message.Args...)
			} else {
				fmt.Printf("Unknown JavaScript method: %s(%v)\n", message.Call, message.Args)
			}
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

	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Println(err)
			return
		}
		client := &WebsocketClient{hub: hub, conn: conn, send: make(chan []byte, 256)}
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
			}
		}
	}()

	return hub
}
