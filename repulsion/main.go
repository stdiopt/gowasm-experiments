//Wasming
// compile: GOOS=js GOARCH=wasm go1.11beta1 build -o main.wasm ./main.go
package main

import (
	"fmt"
	"log"
	"math"
	"math/rand"
	"strconv"
	"syscall/js"
)

var (
	width      float64
	height     float64
	mousePos   [2]float64
	ctx        js.Value
	lineDistSq float64 = 100 * 100
)

func main() {

	// Init Canvas stuff
	doc := js.Global().Get("document")
	canvasEl := doc.Call("getElementById", "mycanvas")
	width = doc.Get("body").Get("clientWidth").Float()
	height = doc.Get("body").Get("clientHeight").Float()
	canvasEl.Call("setAttribute", "width", width)
	canvasEl.Call("setAttribute", "height", height)
	ctx = canvasEl.Call("getContext", "2d")

	done := make(chan struct{}, 0)

	dt := DotThing{speed: 160}

	mouseMoveEvt := js.NewCallback(func(args []js.Value) {
		e := args[0]
		mousePos[0] = e.Get("clientX").Float()
		mousePos[1] = e.Get("clientY").Float()
	})
	defer mouseMoveEvt.Release()

	countChangeEvt := js.NewCallback(func(args []js.Value) {
		evt := args[0]
		intVal, err := strconv.Atoi(evt.Get("target").Get("value").String())
		if err != nil {
			log.Println("Invalid value", err)
			return
		}
		dt.SetNDots(intVal)
	})
	defer countChangeEvt.Release()

	speedInputEvt := js.NewCallback(func(args []js.Value) {
		evt := args[0]
		fval, err := strconv.ParseFloat(evt.Get("target").Get("value").String(), 64)
		if err != nil {
			log.Println("Invalid value", err)
			return
		}
		dt.speed = fval
	})
	defer speedInputEvt.Release()

	// Handle mouse
	doc.Call("addEventListener", "mousemove", mouseMoveEvt)
	doc.Call("getElementById", "count").Call("addEventListener", "change", countChangeEvt)
	doc.Call("getElementById", "speed").Call("addEventListener", "input", speedInputEvt)

	dt.SetNDots(100)
	dt.lines = false
	var renderFrame js.Callback
	var tmark float64
	var markCount = 0
	var tdiffSum float64

	renderFrame = js.NewCallback(func(args []js.Value) {
		now := args[0].Float()
		tdiff := now - tmark
		tdiffSum += now - tmark
		markCount++
		if markCount > 10 {
			doc.Call("getElementById", "fps").Set("innerHTML", fmt.Sprintf("FPS: %.01f", 1000/(tdiffSum/float64(markCount))))
			tdiffSum, markCount = 0, 0
		}
		tmark = now

		// Pool window size to handle resize
		curBodyW := doc.Get("body").Get("clientWidth").Float()
		curBodyH := doc.Get("body").Get("clientHeight").Float()
		if curBodyW != width || curBodyH != height {
			width, height = curBodyW, curBodyH
			canvasEl.Set("width", width)
			canvasEl.Set("height", height)
		}
		dt.Update(tdiff / 1000)

		js.Global().Call("requestAnimationFrame", renderFrame)
	})
	defer renderFrame.Release()

	// Start running
	js.Global().Call("requestAnimationFrame", renderFrame)

	<-done

}

// DotThing manager
type DotThing struct {
	dots  []*Dot
	lines bool
	speed float64
}

// Update updates the dot positions and draws
func (dt *DotThing) Update(dtTime float64) {
	if dt.dots == nil {
		return
	}
	ctx.Call("clearRect", 0, 0, width, height)

	// Update
	for i, dot := range dt.dots {
		if dot.pos[0] < dot.size {
			dot.pos[0] = dot.size
			dot.dir[0] *= -1
		}
		if dot.pos[0] > width-dot.size {
			dot.pos[0] = width - dot.size
			dot.dir[0] *= -1
		}

		if dot.pos[1] < dot.size {
			dot.pos[1] = dot.size
			dot.dir[1] *= -1
		}

		if dot.pos[1] > height-dot.size {
			dot.pos[1] = height - dot.size
			dot.dir[1] *= -1
		}

		mdx := mousePos[0] - dot.pos[0]
		mdy := mousePos[1] - dot.pos[1]
		d := math.Sqrt(mdx*mdx + mdy*mdy)
		if d < 200 {
			dInv := 1 - d/200
			dot.dir[0] += (-mdx / d) * dInv * 8
			dot.dir[1] += (-mdy / d) * dInv * 8
		}
		for j, dot2 := range dt.dots {
			if i == j {
				continue
			}
			mx := dot2.pos[0] - dot.pos[0]
			my := dot2.pos[1] - dot.pos[1]
			d := math.Sqrt(mx*mx + my*my)
			if d < 100 {
				dInv := 1 - d/100
				dot.dir[0] += (-mx / d) * dInv
				dot.dir[1] += (-my / d) * dInv
			}
		}
		dot.dir[0] *= 0.1 //friction
		dot.dir[1] *= 0.1 //friction

		dot.pos[0] += dot.dir[0] * dt.speed * dtTime * 10
		dot.pos[1] += dot.dir[1] * dt.speed * dtTime * 10

		ctx.Set("globalAlpha", 0.5)
		ctx.Call("beginPath")
		ctx.Set("fillStyle", fmt.Sprintf("#%06x", dot.color))
		ctx.Set("strokeStyle", fmt.Sprintf("#%06x", dot.color))
		ctx.Set("lineWidth", dot.size)
		ctx.Call("arc", dot.pos[0], dot.pos[1], dot.size, 0, 2*math.Pi)
		ctx.Call("fill")

	}
}

// SetNDots reinitializes dots with n size
func (dt *DotThing) SetNDots(n int) {
	dt.dots = make([]*Dot, n)
	for i := 0; i < n; i++ {
		dt.dots[i] = &Dot{
			pos: [2]float64{
				rand.Float64() * width,
				rand.Float64() * height,
			},
			dir: [2]float64{
				rand.NormFloat64(),
				rand.NormFloat64(),
			},
			color: uint32(rand.Intn(0xFFFFFF)),
			size:  10,
		}
	}
}

// Dot represents a dot ...
type Dot struct {
	pos   [2]float64
	dir   [2]float64
	color uint32
	size  float64
}
