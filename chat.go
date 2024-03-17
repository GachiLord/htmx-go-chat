package main

import (
	"bytes"
	"encoding/json"
	"github.com/google/uuid"
	"golang.org/x/net/websocket"
	"html/template"
	"time"
)

// room

type RoomState struct {
	Name string
}

type RoomChannels struct {
	Join      chan *websocket.Conn
	Broadcast chan []byte
	TryExpire chan chan bool
}

type Room struct {
	Channels RoomChannels
	State    RoomState
}

type roomMessage struct {
	Message string `json:"message"`
	HEADERS map[string]string
}

// hub

// operations
type Join struct {
	RoomId string
	Conn   *websocket.Conn
	Result chan chan []byte
}

type Create struct {
	Room   RoomState
	Result chan string
}

type LookupResult struct {
	Ok   bool
	Room RoomState
}

type Lookup struct {
	RoomId string
	Result chan LookupResult
}

type Hub struct {
	Join    chan Join
	Create  chan Create
	Lookup  chan Lookup
	Cleanup chan bool
}

func NewHub() *Hub {
	return &Hub{
		Join:    make(chan Join),
		Create:  make(chan Create),
		Lookup:  make(chan Lookup),
		Cleanup: make(chan bool),
	}
}

// tasks

func RunHub(hub *Hub) {
	// create rooms
	rooms := make(map[string]Room)
	// spawn cleanup tasks
	go func() {
		for {
			// check for unused rooms every 30 minutes
			time.Sleep(3 * 10 * time.Minute)
			hub.Cleanup <- true
		}
	}()
	// proceed msgs
	for {
		select {
		case msg := <-hub.Join:
			room, ok := rooms[msg.RoomId]
			if ok {
				room.Channels.Join <- msg.Conn
				msg.Result <- room.Channels.Broadcast
			} else {
				msg.Result <- nil
			}
		case msg := <-hub.Create:
			room := Room{
				State: msg.Room,
				Channels: RoomChannels{
					Broadcast: make(chan []byte),
					Join:      make(chan *websocket.Conn),
					TryExpire: make(chan chan bool, 1),
				},
			}
			id := uuid.NewString()
			rooms[id] = room
			go runRoom(room.Channels)
			msg.Result <- id
		case msg := <-hub.Lookup:
			room, ok := rooms[msg.RoomId]
			if ok {
				msg.Result <- LookupResult{
					Ok:   true,
					Room: room.State,
				}
			} else {
				msg.Result <- LookupResult{
					Ok: false,
				}
			}
		case <-hub.Cleanup:
			for id, room := range rooms {
				result := make(chan bool, 1)
				room.Channels.TryExpire <- result
				if <-result {
					delete(rooms, id)
				}
			}
		}
	}
}

func runRoom(cns RoomChannels) {
	clients := make(map[*websocket.Conn]bool)
	for {
		select {
		case msg := <-cns.Broadcast:
			for client := range clients {
				var parsedMsg roomMessage
				err := json.Unmarshal(msg, &parsedMsg)

				if err == nil {
					buf := bytes.NewBuffer([]byte{})
					tmpl := template.Must(template.ParseFiles(tmplDir + "components/chatMessage.html"))
					err = tmpl.Execute(buf, parsedMsg.Message)

					if err != nil {
						continue
					}

					_, err := client.Write(buf.Bytes())
					if err != nil {
						delete(clients, client)
					}
				}
			}
		case msg := <-cns.Join:
			clients[msg] = true
		case msg := <-cns.TryExpire:
			// delete disconnected clients
			for client := range clients {
				var empty = make([]byte, 0)
				if _, err := client.Write(empty); err != nil {
					delete(clients, client)
				}
			}
			// if there are no clients, stop the task
			hasNoClients := len(clients) == 0
			msg <- hasNoClients
			if hasNoClients {
				break
			}
		}
	}
}
