//Wasming
// compile: GOOS=js GOARCH=wasm go build -o main.wasm ./main.go
package main

import (
	"math"
	"syscall/js"

	"github.com/lucasb-eyer/go-colorful"
)

var (
	mousePos [2]float64
	ctx      js.Value
)

func main() {

	doc := js.Global().Get("document")
	canvasEl := js.Global().Get("document").Call("getElementById", "mycanvas")

	bodyW := doc.Get("body").Get("clientWidth").Float()
	bodyH := doc.Get("body").Get("clientHeight").Float()
	canvasEl.Set("width", bodyW)
	canvasEl.Set("height", bodyH)
	ctx = canvasEl.Call("getContext", "2d")

	done := make(chan struct{}, 0)

	colorRot := float64(0)
	curPos := []float64{100, 75}

	mouseMoveEvt := js.NewCallback(func(args []js.Value) {
		e := args[0]
		mousePos[0] = e.Get("clientX").Float()
		mousePos[1] = e.Get("clientY").Float()
	})
	defer mouseMoveEvt.Release()

	doc.Call("addEventListener", "mousemove", mouseMoveEvt)

	var renderFrame js.Callback
	renderFrame = js.NewCallback(func(args []js.Value) {
		// Handle window resizing
		curBodyW := doc.Get("body").Get("clientWidth").Float()
		curBodyH := doc.Get("body").Get("clientHeight").Float()
		if curBodyW != bodyW || curBodyH != bodyH {
			bodyW, bodyH = curBodyW, curBodyH
			canvasEl.Set("width", bodyW)
			canvasEl.Set("height", bodyH)
		}
		moveX := (mousePos[0] - curPos[0]) * 0.02
		moveY := (mousePos[1] - curPos[1]) * 0.02

		curPos[0] += moveX
		curPos[1] += moveY

		colorRot = float64(int(colorRot+1) % 360)
		ctx.Set("fillStyle", colorful.Hsv(colorRot, 1, 1).Hex())
		ctx.Call("beginPath")
		ctx.Call("arc", curPos[0], curPos[1], 50, 0, 2*math.Pi)
		ctx.Call("fill")

		js.Global().Call("requestAnimationFrame", renderFrame)
	})
	defer renderFrame.Release()

	js.Global().Call("requestAnimationFrame", renderFrame)

	<-done
}
