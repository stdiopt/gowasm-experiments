//Wasming
// compile: GOOS=js GOARCH=wasm go build -o main.wasm ./main.go
package main

import (
	"encoding/json"
	"image/color"
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

	colorHex  string
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

		mouseDown := false
		mouseDownEvt := js.NewCallback(func(args []js.Value) {
			e := args[0]
			if e.Get("target") != c.canvasEl || e.Get("buttons").Float() != 1 {
				return
			}

			if !e.Get("shiftKey").Bool() {
				c.mousePos[0] = e.Get("pageX").Float()
				c.mousePos[1] = e.Get("pageY").Float()
			}
			c.drawAtPointer(e)
			mouseDown = true
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
		c.doc.Call("addEventListener", "mousemove", mouseMoveEvt)
		c.doc.Call("addEventListener", "mousedown", mouseDownEvt)
		c.doc.Call("addEventListener", "mouseup", mouseUpEvt)

		<-c.done
	}()
}
func (c *CanvasClient) drawAtPointer(e js.Value) {
	lastPos := c.mousePos

	c.mousePos[0] = e.Get("pageX").Float()
	c.mousePos[1] = e.Get("pageY").Float()

	col, _ := colorful.Hex(c.colorHex) // Ignore error
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
