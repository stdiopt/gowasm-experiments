//Wasming
package main

import (
	"log"
	"math"
	"syscall/js"

	"github.com/lucasb-eyer/go-colorful"
)

func main() {

	doc := js.Global().Get("document")
	canvasEl := js.Global().Get("document").Call("getElementById", "mycanvas")

	bodyW := doc.Get("body").Get("clientWidth").Float()
	bodyH := doc.Get("body").Get("clientHeight").Float()

	canvasEl.Call("setAttribute", "width", bodyW)
	canvasEl.Call("setAttribute", "height", bodyH)

	if canvasEl == js.Undefined() {
		log.Println("Canvas is undefined")
		return
	}

	ctx := canvasEl.Call("getContext", "2d")

	colorRot := float64(0)
	curPos := []float64{100, 75}
	targetPos := []float64{100, 100}

	done := make(chan struct{}, 0)

	mouseEvent := js.NewCallback(func(args []js.Value) {
		e := args[0]
		targetPos[0] = e.Get("clientX").Float() - canvasEl.Get("offsetLeft").Float()
		targetPos[1] = e.Get("clientY").Float() - canvasEl.Get("offsetTop").Float()
	})

	doc.Call("addEventListener", "mousemove", mouseEvent)

	var renderFrame js.Callback
	renderFrame = js.NewCallback(func(args []js.Value) {
		// Handle window resizing
		curBodyW := doc.Get("body").Get("clientWidth").Float()
		curBodyH := doc.Get("body").Get("clientHeight").Float()
		if curBodyW != bodyW || curBodyH != bodyH {
			bodyW, bodyH = curBodyW, curBodyH

			canvasEl.Call("setAttribute", "width", bodyW)
			canvasEl.Call("setAttribute", "height", bodyH)
		}
		moveX := (targetPos[0] - curPos[0]) * 0.02
		moveY := (targetPos[1] - curPos[1]) * 0.02

		curPos[0] += moveX
		curPos[1] += moveY

		colorRot = float64(int(colorRot+1) % 360)
		ctx.Set("fillStyle", colorful.Hsv(colorRot, 1, 1).Hex())
		ctx.Call("beginPath")
		ctx.Call("arc", curPos[0], curPos[1], 50, 0, 2*math.Pi)
		ctx.Call("fill")

		js.Global().Call("requestAnimationFrame", renderFrame)
	})

	js.Global().Call("requestAnimationFrame", renderFrame)

	<-done
}
