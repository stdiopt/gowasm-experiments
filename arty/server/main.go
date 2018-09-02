package main

import (
	"image/color"
	"log"
	"net/http"
	"sync"

	"github.com/stdiopt/gowasm-experiments/arty/painter"

	"github.com/gorilla/websocket"
)

func main() {
	addr := ":4444"
	log.Println("Listening at ", addr)
	server, err := NewCanvasServer(1920, 1080)
	if err != nil {
		log.Fatal("error starting canvas server", err)
	}
	http.ListenAndServe(addr, server)
}

type CanvasServer struct {
	painter *painter.BufPainter
	clients sync.Map
}

func NewCanvasServer(w, h int) (*CanvasServer, error) {
	p, err := painter.New()
	if err != nil {
		return nil, err
	}
	p.Init(painter.InitOP{Width: w, Height: h})

	p.HandleOP(painter.TextOP{
		Color: color.RGBA{R: 0, G: 0, B: 0, A: 255},
		X:     10.0,
		Y:     10.0,
		Text:  "Hello world",
	})
	return &CanvasServer{p, sync.Map{}}, nil
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

func (s *CanvasServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.Println("Receiving connection from:", r.RemoteAddr)
	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("upgrade err", err)
		return
	}

	c.WriteJSON(painter.Message{painter.InitOP{
		Width:  s.painter.Width(),
		Height: s.painter.Height(),
		Data:   s.painter.ImageData(),
	}})
	if err != nil {
		log.Println("sending msg error")
		return
	}

	ncli := &Cli{conn: c}
	s.clients.Store(ncli, true)
	defer func() {
		c.Close()
		s.clients.Delete(ncli)
	}()
	for {
		mt, message, err := c.ReadMessage()
		if err != nil {
			log.Println("Bye bye", r.RemoteAddr)
			return
		}
		if mt != websocket.TextMessage {
			continue
		}
		// draw in server
		err = s.painter.HandleRaw(message)
		if err != nil {
			log.Println("what?", err)
		}

		// Broadcast to other clients
		s.clients.Range(func(key, value interface{}) bool {
			cl := key.(*Cli)
			if cl == ncli {
				return true
			}
			err := cl.send(mt, message)
			if err != nil {
				log.Println("Erro: sending to cli", err)
			}
			return true
		})

	}
}

// Cli concurrent safe client
type Cli struct {
	sync.Mutex
	conn *websocket.Conn
}

func (c *Cli) send(mt int, msg []byte) error {
	c.Lock()
	defer c.Unlock()
	return c.conn.WriteMessage(mt, msg)
}
