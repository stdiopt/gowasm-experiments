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
	// will hold js part of the image
	byteArray js.Value

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
	c.byteArray = js.Global().Get("Uint8Array").New(1 * 4)
	c.painter.OnInit = func(m painter.InitOP) {
		c.im = c.ctx.Call("createImageData", m.Width, m.Height)
		c.byteArray = js.Global().Get("Uint8Array").New(m.Width * m.Height * 4)
		c.SetStatus("connected")
		c.initEvents()
	}
}

func (c *CanvasClient) initFrameUpdate() {
	// Hold the callbacks without blocking
	go func() {
		var renderFrame js.Func
		renderFrame = js.FuncOf(func(this js.Value, args []js.Value) interface{} {
			c.draw()
			js.Global().Call("requestAnimationFrame", renderFrame)
			return nil
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
		onopen := js.FuncOf(func(this js.Value, args []js.Value) interface{} {
			c.SetStatus("receiving... (it takes some time)")
			return nil
		})
		defer onopen.Release()
		onmessage := js.FuncOf(func(this js.Value, args []js.Value) interface{} {
			c.painter.HandleRaw([]byte(args[0].Get("data").String()))
			return nil
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
		colorEvt := js.FuncOf(func(this js.Value, args []js.Value) interface{} {
			e := args[0]
			c.colorHex = e.Get("target").Get("value").String()
			return nil
		})
		szEvt := js.FuncOf(func(this js.Value, args []js.Value) interface{} {
			e := args[0]
			v, _ := strconv.ParseFloat(e.Get("target").Get("value").String(), 64)
			c.lineWidth = v
			return nil
		})
		defer szEvt.Release()

		c.doc.Call("getElementById", "color").Call("addEventListener", "change", colorEvt)
		c.doc.Call("getElementById", "size").Call("addEventListener", "change", szEvt)

		// Input events
		mouseDown := false
		mouseDownEvt := js.FuncOf(func(this js.Value, args []js.Value) interface{} {
			e := args[0]
			if e.Get("target") != c.canvasEl || e.Get("buttons").Float() != 1 {
				return nil
			}
			mouseDown = true
			if !e.Get("shiftKey").Bool() {
				c.lastPos.x = e.Get("pageX").Float()
				c.lastPos.y = e.Get("pageY").Float()
				c.textOff = pos{} // reset
				return nil
			}
			c.drawAtPointer(e)
			return nil
		})
		defer mouseDownEvt.Release()

		mouseUpEvt := js.FuncOf(func(this js.Value, args []js.Value) interface{} {
			mouseDown = false
			return nil
		})
		defer mouseUpEvt.Release()

		mouseMoveEvt := js.FuncOf(func(this js.Value, args []js.Value) interface{} {
			if !mouseDown {
				return nil
			}
			c.drawAtPointer(args[0])
			return nil
		})

		keyPressEvt := js.FuncOf(func(this js.Value, args []js.Value) interface{} {
			e := args[0]
			e.Call("preventDefault")
			key := e.Get("key").String()
			if key == "Enter" {
				c.textOff.x = 0
				c.textOff.y += (c.lineWidth + 10)
			}
			if len(key) != 1 {
				return nil
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
				return nil
			}
			c.ws.Call("send", string(buf))
			return nil

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
	// Needs to be a Uint8Array while image data have Uint8ClampedArray
	js.CopyBytesToJS(c.byteArray, c.painter.ImageData())
	c.im.Get("data").Call("set", c.byteArray)
	c.ctx.Call("putImageData", c.im, 0, 0)
}
