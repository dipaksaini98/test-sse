package main

import (
	"fmt"
	"log"
	"net/http"
	"time"
)

var msgChan chan *Message

type Message struct {
	SID string
	Msg string
}

var clients = make(map[string]http.ResponseWriter)

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

func main() {
	router := http.NewServeMux()

	router.HandleFunc("/event", sseHandler)
	router.HandleFunc("/time", getTime)

	log.Fatal(http.ListenAndServe(":3000", router))
}
