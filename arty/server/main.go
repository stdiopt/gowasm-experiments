package main

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"

	"github.com/stdiopt/gowasm-experiments/arty/painter"

	"github.com/gorilla/websocket"
)

func main() {
	addr := ":4444"
	log.Println("Listening at ", addr)
	http.ListenAndServe(addr, NewCanvasServer(1920, 1080))
}

type CanvasServer struct {
	painter *painter.BufPainter
	clients sync.Map
}

func NewCanvasServer(w, h int) *CanvasServer {
	p := painter.New()
	p.Init(painter.InitOP{Width: w, Height: h})
	return &CanvasServer{p, sync.Map{}}
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

	// send the current state (image buf)
	initOp := painter.InitOP{
		Width:  s.painter.Width(),
		Height: s.painter.Height(),
		Data:   s.painter.ImageData(),
	}
	buf, err := json.Marshal(initOp)
	if err != nil {
		log.Println("msg encoding error")
		return
	}

	err = c.WriteMessage(websocket.TextMessage, buf)
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
