//Wasming
// compile: GOOS=js GOARCH=wasm go build -o main.wasm ./main.go
package main

import (
	"encoding/json"
	"image/color"
	"log"
	"strconv"
	"syscall/js"

	"github.com/stdiopt/gowasm-experiments/arty/painter"

	"github.com/lucasb-eyer/go-colorful"
)

func main() {
	c, err := NewCanvasClient("wss:/arty.us.hexasoftware.com")

	if err != nil {
		log.Fatal("could not start", err)
	}
	defer c.Close()
	c.Start()
}

type pos struct {
	x, y float64
}

type CanvasClient struct {
	done    chan struct{}
	painter *painter.BufPainter
	addr    string

	doc      js.Value
	canvasEl js.Value
	ctx      js.Value
	ws       js.Value
	im       js.Value

	colorHex  string
	lineWidth float64

	textOff pos
	lastPos pos
	width   float64
	height  float64
}

func NewCanvasClient(addr string) (*CanvasClient, error) {
	done := make(chan struct{})

	painter, err := painter.New()
	if err != nil {
		return nil, err
	}
	return &CanvasClient{
		done:      done,
		painter:   painter,
		addr:      addr,
		lineWidth: 10,
	}, nil
}

func (c *CanvasClient) Start() {
	c.initCanvas()
	c.initFrameUpdate()
	c.initConnection()

	<-c.done
}

func (c *CanvasClient) Close() {
	close(c.done)
}

func (c *CanvasClient) initCanvas() {
	c.doc = js.Global().Get("document")
	c.canvasEl = c.doc.Call("getElementById", "mycanvas")
	c.width = c.canvasEl.Get("width").Float()
	c.height = c.canvasEl.Get("height").Float()
	c.ctx = c.canvasEl.Call("getContext", "2d")
	c.im = c.ctx.Call("createImageData", 1, 1)
	c.painter.OnInit = func(m painter.InitOP) {
		c.im = c.ctx.Call("createImageData", m.Width, m.Height)
		c.SetStatus("connected")
		c.initEvents()
	}
}

func (c *CanvasClient) initFrameUpdate() {
	// Hold the callbacks without blocking
	go func() {
		var renderFrame js.Callback
		renderFrame = js.NewCallback(func(args []js.Value) {
			c.draw()
			js.Global().Call("requestAnimationFrame", renderFrame)
		})
		defer renderFrame.Release()
		js.Global().Call("requestAnimationFrame", renderFrame)
		<-c.done
	}()
}

func (c *CanvasClient) initConnection() {
	go func() {
		c.SetStatus("connecting...")
		c.ws = js.Global().Get("WebSocket").New(c.addr)
		onopen := js.NewCallback(func(args []js.Value) {
			c.SetStatus("receiving...")
		})
		defer onopen.Release()
		onmessage := js.NewCallback(func(args []js.Value) {
			c.painter.HandleRaw([]byte(args[0].Get("data").String()))
		})
		defer onmessage.Release()
		c.ws.Set("onopen", onopen)
		c.ws.Set("onmessage", onmessage)

		<-c.done
	}()
}
func (c *CanvasClient) initEvents() {
	go func() {
		// DOM events
		colorEvt := js.NewCallback(func(args []js.Value) {
			e := args[0]
			c.colorHex = e.Get("target").Get("value").String()
		})
		szEvt := js.NewCallback(func(args []js.Value) {
			e := args[0]
			v, _ := strconv.ParseFloat(e.Get("target").Get("value").String(), 64)
			c.lineWidth = v
		})
		defer szEvt.Release()

		c.doc.Call("getElementById", "color").Call("addEventListener", "change", colorEvt)
		c.doc.Call("getElementById", "size").Call("addEventListener", "change", szEvt)

		// Input events
		mouseDown := false
		mouseDownEvt := js.NewEventCallback(0, func(e js.Value) {
			if e.Get("target") != c.canvasEl || e.Get("buttons").Float() != 1 {
				return
			}
			mouseDown = true
			if !e.Get("shiftKey").Bool() {
				c.lastPos.x = e.Get("pageX").Float()
				c.lastPos.y = e.Get("pageY").Float()
				c.textOff = pos{} // reset
				return
			}
			c.drawAtPointer(e)
		})
		defer mouseDownEvt.Release()

		mouseUpEvt := js.NewCallback(func(args []js.Value) {
			mouseDown = false
		})
		defer mouseUpEvt.Release()

		mouseMoveEvt := js.NewCallback(func(args []js.Value) {
			if !mouseDown {
				return
			}
			c.drawAtPointer(args[0])
		})

		keyPressEvt := js.NewEventCallback(js.PreventDefault, func(e js.Value) {
			key := e.Get("key").String()
			if key == "Enter" {
				c.textOff.x = 0
				c.textOff.y += (c.lineWidth + 10)
			}
			if len(key) != 1 {
				return
			}
			col, _ := colorful.Hex(c.colorHex) // Ignore error
			op := painter.TextOP{
				color.RGBA{uint8(col.R * 255), uint8(col.G * 255), uint8(col.B * 255), 255},
				c.lineWidth + 6,
				c.lastPos.x + c.textOff.x, c.lastPos.y + c.textOff.y,
				key,
			}
			c.textOff.x += (c.lineWidth + 10) * 0.6

			c.painter.HandleOP(op)
			buf, err := json.Marshal(painter.Message{op})
			if err != nil {
				return
			}
			c.ws.Call("send", string(buf))

		})
		defer keyPressEvt.Release()
		c.doc.Call("addEventListener", "mousemove", mouseMoveEvt)
		c.doc.Call("addEventListener", "mousedown", mouseDownEvt)
		c.doc.Call("addEventListener", "mouseup", mouseUpEvt)
		c.doc.Call("addEventListener", "keypress", keyPressEvt)

		<-c.done
	}()
}
func (c *CanvasClient) drawAtPointer(e js.Value) {
	lastPos := c.lastPos

	c.lastPos.x = e.Get("pageX").Float()
	c.lastPos.y = e.Get("pageY").Float()

	col, _ := colorful.Hex(c.colorHex) // Ignore error
	op := painter.LineOP{
		color.RGBA{uint8(col.R * 255), uint8(col.G * 255), uint8(col.B * 255), 255},
		c.lineWidth,
		lastPos.x, lastPos.y,
		c.lastPos.x, c.lastPos.y,
	}
	c.painter.HandleOP(op)

	buf, err := json.Marshal(painter.Message{op})
	if err != nil {
		return
	}
	c.ws.Call("send", string(buf))

}
func (c *CanvasClient) SetStatus(txt string) {
	c.doc.Call("getElementById", "status").Set("innerHTML", txt)
}
func (c *CanvasClient) draw() {
	// golang buffer
	ta := js.TypedArrayOf(c.painter.ImageData())
	c.im.Get("data").Call("set", ta)
	ta.Release()
	c.ctx.Call("putImageData", c.im, 0, 0)
}
