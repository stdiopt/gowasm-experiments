//Wasming
// compile: gopherjs build -o main.js ./main.go
package main

import (
	"fmt"
	"log"
	"math"
	"math/rand"
	"strconv"

	"github.com/gopherjs/gopherjs/js"
)

var (
	width      float64
	height     float64
	mousePos   [2]float64
	ctx        *js.Object
	lineDistSq float64 = 100 * 100
)

func main() {

	// Init Canvas stuff
	doc := js.Global.Get("document")
	canvasEl := doc.Call("getElementById", "mycanvas")
	width = doc.Get("body").Get("clientWidth").Float()
	height = doc.Get("body").Get("clientHeight").Float()
	canvasEl.Call("setAttribute", "width", width)
	canvasEl.Call("setAttribute", "height", height)
	ctx = canvasEl.Call("getContext", "2d")

	done := make(chan struct{}, 0)

	// Handle mouse
	doc.Call("addEventListener", "mousemove", func(evt *js.Object) {
		mousePos[0] = evt.Get("clientX").Float()
		mousePos[1] = evt.Get("clientY").Float()
	})

	dt := DotThing{speed: 160}

	// Event Handlers for DotThing
	doc.Call("getElementById", "count").Call("addEventListener", "change", func(evt *js.Object) {
		intVal, err := strconv.Atoi(evt.Get("target").Get("value").String())
		if err != nil {
			log.Println("Invalid value", err)
			return
		}
		dt.SetNDots(intVal)
	})
	doc.Call("getElementById", "speed").Call("addEventListener", "input", func(evt *js.Object) {
		fval, err := strconv.ParseFloat(evt.Get("target").Get("value").String(), 64)
		if err != nil {
			log.Println("Invalid value", err)
			return
		}
		dt.speed = fval
	})
	doc.Call("getElementById", "lines").Call("addEventListener", "change", func(evt *js.Object) {
		dt.lines = evt.Get("target").Get("checked").Bool()
	})

	dt.SetNDots(100)
	dt.lines = false
	var renderFrame func(dtTime *js.Object)
	var tmark float64 //= float64(time.Now().UnixNano() / 1000000)
	var markCount = 0
	var tdiffSum float64

	renderFrame = func(dtTime *js.Object) {
		now := dtTime.Float() //float64(time.Now().UnixNano() / 1000000) //args[0].Float()
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

		js.Global.Call("requestAnimationFrame", renderFrame)
	}

	// Start running
	js.Global.Call("requestAnimationFrame", renderFrame)

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
		dir := [2]float64{}
		// Bounce
		if dot.pos[0] < 0 {
			dot.pos[0] = 0
			dot.dir[0] *= -1
		}
		if dot.pos[0] > width {
			dot.pos[0] = width
			dot.dir[0] *= -1
		}

		if dot.pos[1] < 0 {
			dot.pos[1] = 0
			dot.dir[1] *= -1
		}

		if dot.pos[1] > height {
			dot.pos[1] = height
			dot.dir[1] *= -1
		}
		dir[0] = dot.dir[0]
		dir[1] = dot.dir[1]

		ctx.Set("globalAlpha", 0.5)
		ctx.Call("beginPath")
		ctx.Set("fillStyle", fmt.Sprintf("#%06x", dot.color))
		ctx.Set("strokeStyle", fmt.Sprintf("#%06x", dot.color))
		ctx.Set("lineWidth", dot.size)
		ctx.Call("arc", dot.pos[0], dot.pos[1], dot.size, 0, 2*math.Pi)
		ctx.Call("fill")

		mdx := mousePos[0] - dot.pos[0]
		mdy := mousePos[1] - dot.pos[1]
		d := math.Sqrt(mdx*mdx + mdy*mdy)
		if d < 200 {
			ctx.Set("globalAlpha", 1-d/200)
			ctx.Call("beginPath")
			ctx.Call("moveTo", dot.pos[0], dot.pos[1])
			ctx.Call("lineTo", mousePos[0], mousePos[1])
			ctx.Call("stroke")
			if d > 100 { // move towards mouse
				dir[0] = (mdx / d) * 2
				dir[1] = (mdy / d) * 2
			} else { // do not move
				dir[0] = 0
				dir[1] = 0
			}
		}

		if dt.lines {
			for _, dot2 := range dt.dots[i+1:] {
				mx := dot2.pos[0] - dot.pos[0]
				my := dot2.pos[1] - dot.pos[1]
				d := mx*mx + my*my
				if d < lineDistSq {
					ctx.Set("globalAlpha", 1-d/lineDistSq)
					ctx.Call("beginPath")
					ctx.Call("moveTo", dot.pos[0], dot.pos[1])
					ctx.Call("lineTo", dot2.pos[0], dot2.pos[1])
					ctx.Call("stroke")
				}
			}
		}

		dot.pos[0] += dir[0] * dt.speed * dtTime
		dot.pos[1] += dir[1] * dt.speed * dtTime
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
