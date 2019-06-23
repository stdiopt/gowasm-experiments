// +build js

package main

import (
	"fmt"
	"math"
	"math/rand"
	"syscall/js"
)

const (
	nPoints   = 20
	hexE      = 30
	lifeScale = 0.01
	speed     = 2.5
	alpha     = 0.991
	deg2rad   = 0.017453292 // pi/180
)

var (
	hexH = math.Sqrt(3) * hexE
	hexD = float64(math.Round(3 * hexE))
)

func main() {
	ht := hexThing{}
	ht.SetNDots(nPoints)
	ht.start()

}

type hexThing struct {
	dots []*dot

	doc js.Value
	ctx js.Value

	backCtx []js.Value
	curCtx  int

	width  float64
	height float64
	fcount int

	// callbacks
}

func (ht *hexThing) start() {
	ht.doc = js.Global().Get("document")
	canvasEl := ht.doc.Call("getElementById", "mycanvas")
	ht.ctx = canvasEl.Call("getContext", "2d")

	// Create 2 backbuffers
	for i := 0; i < 2; i++ {
		cv := ht.doc.Call("createElement", "canvas")
		cv.Set("width", ht.width)
		cv.Set("height", ht.height)
		ctx := cv.Call("getContext", "2d")
		ht.backCtx = append(ht.backCtx, ctx)
	}

	var renderFrame js.Func
	var tmark float64
	var markCount = 0
	var tdiffSum float64

	renderFrame = js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		// FPS thing
		now := args[0].Float()
		tdiff := now - tmark
		tdiffSum += now - tmark
		markCount++
		if markCount > 10 {
			ht.doc.Call("getElementById", "fps").Set("innerHTML", fmt.Sprintf("FPS: %.01f", 1000/(tdiffSum/float64(markCount))))
			tdiffSum, markCount = 0, 0
		}
		tmark = now

		// Canvas size handler
		curBodyW := ht.doc.Get("body").Get("clientWidth").Float() / 2
		curBodyH := ht.doc.Get("body").Get("clientHeight").Float() / 2
		if curBodyW != ht.width || curBodyH != ht.height {
			ht.width, ht.height = curBodyW, curBodyH
			canvasEl.Set("width", ht.width)
			canvasEl.Set("height", ht.height)
			// update back buffers too
			for _, cx := range ht.backCtx {
				cv := cx.Get("canvas")
				cv.Set("width", ht.width)
				cv.Set("height", ht.height)
			}
		}

		ht.Update(tdiff / 1000)

		js.Global().Call("requestAnimationFrame", renderFrame)
		return nil
	})

	done := make(chan struct{}, 0)

	js.Global().Call("requestAnimationFrame", renderFrame)
	<-done
}

func (ht *hexThing) Update(dtTime float64) {
	ht.fcount++
	prevCtx := ht.backCtx[ht.curCtx]
	ht.curCtx = (ht.curCtx + 1) % len(ht.backCtx)
	ctx := ht.backCtx[ht.curCtx]

	ctx.Call("clearRect", 0, 0, ht.width, ht.height)
	ctx.Set("globalAlpha", 1)
	filter := "hue-rotate(3deg) grayscale(1%)"
	if ht.fcount%120 == 0 {
		filter += " opacity(0.98)"
	}
	if ht.fcount%8 == 0 {
		filter += " blur(1px)"
	}
	ctx.Set("filter", filter)
	ctx.Call("drawImage", prevCtx.Get("canvas"), 0, 0)
	ctx.Set("filter", "")

	for _, d := range ht.dots {
		d.life -= 0.005
		if d.life < 0 { // Reset
			ht.dotReset(d)
			continue
		}
		d.r += speed
		if d.r >= hexE {
			sign := 1.0
			if rand.Float64()-0.5 < 0 {
				sign = -1.0
			}
			d.dir += 60 * sign
			d.r = 0
		}

		prevPos := d.pos

		d.x += math.Cos(d.dir*deg2rad) * speed
		d.y += math.Sin(d.dir*deg2rad) * speed
		// out of bounds
		if d.x < 0 || d.x > ht.width || d.y < 0 || d.y > ht.height {
			ht.dotReset(d)
			continue
		}
		ctx.Set("globalAlpha", d.life)
		ctx.Set("strokeStyle", "orange")

		ctx.Call("beginPath")
		ctx.Call("moveTo", prevPos.x, prevPos.y)
		ctx.Call("lineTo", d.x, d.y)
		ctx.Call("stroke")
	}

	ht.ctx.Call("clearRect", 0, 0, ht.width, ht.height)
	ht.ctx.Set("globalAlpha", 1)
	ht.ctx.Call("drawImage", ctx.Get("canvas"), 0, 0)

}

func (ht *hexThing) dotReset(d *dot) {
	fH := math.Floor(ht.width / hexH)
	fW := math.Floor(ht.height / hexD)

	*d = dot{
		pos: pos{
			x: math.Floor(rand.Float64()*fH) * hexH,
			y: math.Floor(rand.Float64()*fW) * hexD,
		},
		r:    0,
		dir:  90,
		life: 1 + rand.Float64(),
	}

}
func (ht *hexThing) SetNDots(n int) {
	ht.dots = make([]*dot, n)
	for i := 0; i < n; i++ {
		d := &dot{}
		ht.dotReset(d)
		ht.dots[i] = d
	}
}

type dot struct {
	pos
	r    float64
	dir  float64
	life float64
}

type pos struct {
	x float64
	y float64
}
