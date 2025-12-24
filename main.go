package main

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

var msgChan chan *Message

type Message struct {
	SID string
	Msg string
}

var clients = make(map[string]http.ResponseWriter)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	// IMPORTANT: lock this down for production
	CheckOrigin: func(r *http.Request) bool {
		// // Example: allow same-origin; customize to your domains
		// origin := r.Header.Get("Origin")
		// return origin == "http://127.0.0.1:5500"

		// WARNING: allows all origins; restrict this for production use
		return true
	},
}

func wsHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("upgrade:", err)
		return
	}
	defer conn.Close()

	// Good practice: set limits + timeouts
	conn.SetReadLimit(1 << 20) // 1MB
	_ = conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	conn.SetPongHandler(func(string) error {
		_ = conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	// Ping loop to keep idle connections alive across proxies
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	go func() {
		for range ticker.C {
			_ = conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}()

	for {
		mt, msg, err := conn.ReadMessage()
		if err != nil {
			log.Println("read:", err)
			return
		}
		log.Println(string(msg))
		xyz := "BITS"
		msg = []byte(xyz)
		_ = conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
		if err := conn.WriteMessage(mt, msg); err != nil {
			log.Println("write:", err)
			return
		}
	}
}

func getTime(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")

	sid := r.URL.Query().Get("sid")
	if msgChan != nil {
		msg := time.Now().Format("15:04:05")
		message := &Message{
			SID: sid,
			Msg: msg,
		}
		msgChan <- message
	}
}

func sseHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Println("Client connected!")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	// read sid from request query
	sid := r.URL.Query().Get("sid")
	clients[sid] = w

	msgChan = make(chan *Message)

	defer func() {
		close(msgChan)
		msgChan = nil
		fmt.Println("Client closed connection!")
	}()

	for {
		select {
		case message := <-msgChan:
			fmt.Println(message)
			rw := clients[message.SID]
			flusher, ok := rw.(http.Flusher)
			if !ok {
				fmt.Println("Could not init http.Flusher!")
			}
			fmt.Fprintf(rw, "data: %s\n\n", message.Msg)
			flusher.Flush()
		case <-r.Context().Done():
			fmt.Println("Client closed connection!")
			return
		}
	}

}

func pingPong(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("pong"))
}

func main() {
	router := http.NewServeMux()

	router.HandleFunc("/event", sseHandler)
	router.HandleFunc("/time", getTime)
	router.HandleFunc("/ws", wsHandler)
	router.HandleFunc("/ping", pingPong)
	router.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Congratulations, you found the chamber of secrets!"))
	})

	srv := &http.Server{
		Addr:              ":8080",
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
	}

	log.Println("Server started on port 8080!")
	log.Fatal(srv.ListenAndServe())
}
