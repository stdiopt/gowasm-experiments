//Wasming
// compile: GOOS=js GOARCH=wasm go build -o main.wasm ./main.go
package main

import (
	"encoding/json"
	"fmt"
	"image/color"
	"math"
	"syscall/js"

	"github.com/stdiopt/gowasm-experiments/arty/painter"

	"github.com/lucasb-eyer/go-colorful"
)

func main() {

	c := NewCanvasClient("ws:/us.hexasoftware.com:4444")
	defer c.Close()

	c.Start()
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

	colordeg  float64
	colorsat  float64
	lineWidth float64

	mousePos [2]float64
	width    float64
	height   float64
	status   string
}

func NewCanvasClient(addr string) *CanvasClient {
	done := make(chan struct{})

	painter := painter.New()
	return &CanvasClient{
		done:      done,
		painter:   painter,
		addr:      addr,
		colorsat:  0.2,
		lineWidth: 10,
	}
}

func (c *CanvasClient) Start() {
	c.initCanvas()
	c.initFrameUpdate()
	c.initConnection()
	c.initEvents()

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
		c.status = "connected"
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
		c.status = "connecting..."
		c.ws = js.Global().Get("WebSocket").New(c.addr)
		onopen := js.NewCallback(func(args []js.Value) {
			c.status = "receiving..."
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
		mouseDown := false
		mouseDownEvt := js.NewCallback(func(args []js.Value) {
			mouseDown = true
		})
		defer mouseDownEvt.Release()
		mouseUpEvt := js.NewCallback(func(args []js.Value) {
			mouseDown = false
		})
		defer mouseUpEvt.Release()
		mouseMoveEvt := js.NewCallback(func(args []js.Value) {
			e := args[0]
			lastPos := c.mousePos
			c.mousePos[0] = e.Get("pageX").Float()
			c.mousePos[1] = e.Get("pageY").Float()
			if !mouseDown {
				return
			}

			col := colorful.Hsv(c.colordeg, c.colorsat, 1)
			op := painter.LineOP{
				color.RGBA{uint8(col.R * 255), uint8(col.G * 255), uint8(col.B * 255), 255},
				c.lineWidth,
				lastPos[0], lastPos[1],
				c.mousePos[0], c.mousePos[1],
			}
			c.painter.Line(op)

			buf, err := json.Marshal(&op)
			if err != nil {
				return
			}

			c.ws.Call("send", string(buf))

		})
		keyEvt := js.NewCallback(func(args []js.Value) {
			e := args[0]
			key := e.Get("key").String()
			switch key {
			case "1":
				c.colorsat = math.Max(c.colorsat-0.1, 0)
			case "2":
				c.colorsat = math.Min(c.colorsat+0.1, 1)
			case "3":
				c.colordeg = math.Mod(c.colordeg-1, 360)
			case "4":
				c.colordeg = math.Mod(c.colordeg+1, 360)
			case "5":
				c.lineWidth = math.Max(c.lineWidth-1, 2)
			case "6":
				c.lineWidth = math.Min(c.lineWidth+1, 540)
			}
			if c.colordeg < 0 {
				c.colordeg += 360
			}

		})
		defer keyEvt.Release()
		c.doc.Call("addEventListener", "mousemove", mouseMoveEvt)
		c.doc.Call("addEventListener", "mousedown", mouseDownEvt)
		c.doc.Call("addEventListener", "mouseup", mouseUpEvt)
		c.doc.Call("addEventListener", "keydown", keyEvt)

		<-c.done
	}()
}
func (c *CanvasClient) draw() {

	// golang buffer
	ta := js.TypedArrayOf(c.painter.ImageData())
	c.im.Get("data").Call("set", ta)
	ta.Release()
	c.ctx.Call("putImageData", c.im, 0, 0)

	// bottom status
	c.ctx.Set("fillStyle", colorful.Hsv(c.colordeg, c.colorsat, 1).Hex())
	c.ctx.Call("fillRect", 0, c.height-30, c.width, c.height)
	c.ctx.Set("font", "20px Georgia")
	c.ctx.Set("fillStyle", "black")
	c.ctx.Call("fillText",
		fmt.Sprintf(
			"%-40s Keys (1/2) sat(%.02f) | (3/4) hue(%.02f) (5/6) size(%.02f)",
			c.status, c.colorsat, c.colordeg, c.lineWidth,
		),
		10, c.height-9,
	)
}
