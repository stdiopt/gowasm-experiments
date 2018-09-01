//Wasming
// compile: GOOS=js GOARCH=wasm go build -o main.wasm ./main.go
package main

import (
	"encoding/json"
	"image/color"
	"math"
	"strconv"
	"syscall/js"

	"github.com/stdiopt/gowasm-experiments/arty/painter"

	"github.com/lucasb-eyer/go-colorful"
)

func main() {

	c := NewCanvasClient("wss:/arty.us.hexasoftware.com")
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
		c.SetStatus("connected")
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

		satEvt := js.NewCallback(func(args []js.Value) {
			e := args[0]
			v, _ := strconv.ParseFloat(e.Get("target").Get("value").String(), 64)
			c.colorsat = v
		})
		defer satEvt.Release()
		hueEvt := js.NewCallback(func(args []js.Value) {
			e := args[0]
			v, _ := strconv.ParseFloat(e.Get("target").Get("value").String(), 64)
			c.colordeg = v
		})
		defer hueEvt.Release()
		szEvt := js.NewCallback(func(args []js.Value) {
			e := args[0]
			v, _ := strconv.ParseFloat(e.Get("target").Get("value").String(), 64)
			c.lineWidth = v
		})
		defer szEvt.Release()

		c.doc.Call("getElementById", "sat").Call("addEventListener", "change", satEvt)
		c.doc.Call("getElementById", "hue").Call("addEventListener", "change", hueEvt)
		c.doc.Call("getElementById", "size").Call("addEventListener", "change", szEvt)

		mouseDown := false
		mouseDownEvt := js.NewCallback(func(args []js.Value) {
			e := args[0]
			if e.Get("target") != c.canvasEl {
				return
			}
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
