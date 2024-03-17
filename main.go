package main

import (
	"bytes"
	"golang.org/x/net/websocket"
	"html/template"
	"io"
	"log"
	"net/http"
)

// pages

var basePath = "static/templates/components/base.html"
var baseAppOnlyPath = "static/templates/components/baseAppOnly.html"
var tmplDir = "static/templates/"

func pageNotFound(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNotFound)
	tmpl := template.Must(template.ParseFiles(basePath, tmplDir+"pages/notFound.html"))
	tmpl.Execute(w, nil)
}

func homePage(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		pageNotFound(w)
		return
	}

	w.Header().Add("Vary", "HX-Request")
	w.Header().Add("Cache-control", "max-age=604800, public")

	if r.Header.Get("HX-Boosted") == "true" {
		tmpl := template.Must(template.ParseFiles(baseAppOnlyPath, tmplDir+"pages/home.html"))
		tmpl.Execute(w, nil)
	} else {
		tmpl := template.Must(template.ParseFiles(basePath, tmplDir+"pages/home.html"))
		tmpl.Execute(w, nil)
	}
}

func newChatPage(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Vary", "HX-Request")
	w.Header().Add("Cache-control", "max-age=604800, public")

	if r.Header.Get("HX-Boosted") == "true" {
		tmpl := template.Must(template.ParseFiles(baseAppOnlyPath, tmplDir+"pages/newChat.html", tmplDir+"components/button.html"))
		tmpl.Execute(w, nil)
	} else {
		tmpl := template.Must(template.ParseFiles(basePath, tmplDir+"pages/newChat.html", tmplDir+"components/button.html"))
		tmpl.Execute(w, nil)
	}
}

type chatInfo struct {
	Id   string
	Room RoomState
}

func chatPage(hub *Hub, w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	lookup := Lookup{
		RoomId: id,
		Result: make(chan LookupResult, 1),
	}
	hub.Lookup <- lookup
	res := <-lookup.Result

	if res.Ok == false {
		pageNotFound(w)
		return
	}

	info := chatInfo{
		Id:   id,
		Room: res.Room,
	}

	if r.Header.Get("HX-Boosted") == "true" {
		tmpl := template.Must(template.ParseFiles(baseAppOnlyPath, tmplDir+"pages/chat.html"))
		tmpl.Execute(w, info)
	} else {
		tmpl := template.Must(template.ParseFiles(basePath, tmplDir+"pages/chat.html"))
		tmpl.Execute(w, info)
	}
}

// api

func chatHandler(hub *Hub, w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPut {
		err := r.ParseForm()

		if err != nil || len(r.Form.Get("name")) > 40 {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		createData := Create{
			Room: RoomState{
				Name: r.Form.Get("name"),
			},
			Result: make(chan string),
		}
		hub.Create <- createData
		w.Header().Add("HX-Location", "/chat/"+<-createData.Result)
	} else {
		http.NotFound(w, r)
	}
}

func ws(hub *Hub, c *websocket.Conn) {
	// try to join the room
	joinData := Join{
		RoomId: c.Request().PathValue("id"),
		Result: make(chan chan []byte, 1),
		Conn:   c,
	}
	hub.Join <- joinData
	roomChan := <-joinData.Result
	// if there is no such room, send err msg and close conn
	if roomChan == nil {
		buf := bytes.NewBuffer([]byte{})
		errMsg := template.Must(template.ParseFiles(tmplDir + "components/chatError.html"))
		errMsg.Execute(buf, nil)
		c.Write(buf.Bytes())
		c.Close()
		return
	}
	// send client messages
	for {
		reader, err := c.NewFrameReader()

		if err != nil {
			break
		}

		buf, err := io.ReadAll(reader)

		if err != nil {
			break
		}

		roomChan <- buf
	}
	c.Close()
}

func main() {
	fs := http.FileServer(http.Dir("static/resources"))
	hub := NewHub()
	go RunHub(hub)

	http.HandleFunc("/", homePage)
	http.HandleFunc("/new", newChatPage)
	http.HandleFunc("/chat/{id}", func(w http.ResponseWriter, r *http.Request) {
		chatPage(hub, w, r)
	})
	http.Handle("/resources/", http.StripPrefix("/resources", fs))
	// api
	http.HandleFunc("/api/chat", func(w http.ResponseWriter, r *http.Request) {
		chatHandler(hub, w, r)
	})
	http.Handle("/ws/chat/{id}", websocket.Handler(func(c *websocket.Conn) {
		ws(hub, c)
	}))

	log.Fatal(http.ListenAndServe(":3000", nil))
}
