// Click on canvas to start a polygon
// - Max 8 vertices
// - Only convex polygons
// - Esc cancel polygon

//Wasming
// compile: GOOS=js GOARCH=wasm go build -o main.wasm ./main.go
package main

import (
	"fmt"
	"math"
	"math/rand"
	"strconv"
	"syscall/js"

	// this box2d throws some unexpected panics
	"github.com/ByteArena/box2d"
)

var (
	width      float64
	height     float64
	ctx        js.Value
	simSpeed   float64 = 1
	worldScale float64 = 0.0125 // 1/8
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
	ctx.Call("scale", 1/worldScale, 1/worldScale)

	done := make(chan struct{}, 0)

	world := box2d.MakeB2World(box2d.B2Vec2{X: 0, Y: 9.8})
	var verts []box2d.B2Vec2

	keyUpEvt := js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		e := args[0]
		if e.Get("which").Int() == 27 {
			verts = nil
		}
		return nil
	})
	defer keyUpEvt.Release()
	mouseDownEvt := js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		defer func() {
			// Recovering from possible box2d panic
			if r := recover(); r != nil {
				verts = nil
			}
		}()

		e := args[0]
		if e.Get("target") != canvasEl {
			return nil
		}
		mx := e.Get("clientX").Float() * worldScale
		my := e.Get("clientY").Float() * worldScale
		// Start shape
		if verts == nil {
			verts = []box2d.B2Vec2{box2d.B2Vec2{mx, my}}
			return nil
		}
		dx := mx - verts[0].X
		dy := my - verts[0].Y
		d := math.Sqrt(dx*dx + dy*dy)
		///////
		// if clicked on the single spot we create a ball
		if len(verts) == 1 && d < 10*worldScale {
			obj1 := world.CreateBody(&box2d.B2BodyDef{
				Type:         box2d.B2BodyType.B2_dynamicBody,
				Position:     box2d.B2Vec2{X: mx, Y: my},
				Awake:        true,
				Active:       true,
				GravityScale: 1.0,
			})
			shape := box2d.NewB2CircleShape()
			shape.M_radius = (10 + rand.Float64()*10) * worldScale
			ft := obj1.CreateFixture(shape, 1)
			ft.M_friction = 0.3
			ft.M_restitution = 0.7
			verts = nil
			return nil
		}
		if len(verts) > 2 && d < 10*worldScale || len(verts) == 8 {

			// Seems box2d panics when we create a polygon counterclockwise most
			// likely due to normals and centroids calculations so basically we
			// recover from that panic and invert the polygon and try again
			var center *box2d.B2Vec2
			func() {
				defer func() { recover() }()
				lc := box2d.ComputeCentroid(verts, len(verts))
				center = &lc
			}()
			if center == nil {
				//vert inversion
				verts2 := make([]box2d.B2Vec2, len(verts))
				for i := range verts {
					verts2[len(verts)-1-i] = verts[i]
				}
				verts = verts2
				// Retry
				lc := box2d.ComputeCentroid(verts, len(verts))
				center = &lc
			}

			// translate -center
			for i := range verts {
				verts[i].X -= center.X
				verts[i].Y -= center.Y
			}
			shape := box2d.NewB2PolygonShape()
			shape.Set(verts, len(verts))

			obj := world.CreateBody(&box2d.B2BodyDef{
				Type:         box2d.B2BodyType.B2_dynamicBody,
				Position:     *center,
				Awake:        true,
				Active:       true,
				GravityScale: 1.0,
			})
			fixture := obj.CreateFixture(shape, 10)
			fixture.M_friction = 0.3
			verts = nil
			return nil
		}
		verts = append(verts, box2d.B2Vec2{mx, my})
		return nil
	})
	defer mouseDownEvt.Release()

	speedInputEvt := js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		evt := args[0]
		fval, err := strconv.ParseFloat(evt.Get("target").Get("value").String(), 64)
		if err != nil {
			println("Invalid value", err)
			return nil
		}
		simSpeed = fval
		return nil
	})
	defer speedInputEvt.Release()

	doc.Call("addEventListener", "keyup", keyUpEvt)
	doc.Call("addEventListener", "mousedown", mouseDownEvt)
	doc.Call("getElementById", "speed").Call("addEventListener", "input", speedInputEvt)

	// Floor
	floor := world.CreateBody(&box2d.B2BodyDef{
		Type:     box2d.B2BodyType.B2_kinematicBody,
		Position: box2d.B2Vec2{X: 0, Y: height*worldScale - 20*worldScale},
		Active:   true,
	})
	floorShape := &box2d.B2PolygonShape{}
	floorShape.SetAsBox(width*worldScale, 20*worldScale)
	ft := floor.CreateFixture(floorShape, 1)
	ft.M_friction = 0.3

	// Some Random falling balls
	for i := 0; i < 10; i++ {
		obj1 := world.CreateBody(&box2d.B2BodyDef{
			Type:         box2d.B2BodyType.B2_dynamicBody,
			Position:     box2d.B2Vec2{X: rand.Float64() * width * worldScale, Y: rand.Float64() * height * worldScale},
			Awake:        true,
			Active:       true,
			GravityScale: 1.0,
		})
		shape := box2d.NewB2CircleShape()
		shape.M_radius = 10 * worldScale
		ft := obj1.CreateFixture(shape, 1)
		ft.M_friction = 0.3
		ft.M_restitution = 0.5 // bouncy
	}

	// Draw things
	var renderFrame js.Func
	var tmark float64

	// overall style
	ctx.Set("fillStyle", "rgba(100,150,100,0.4)")
	ctx.Set("strokeStyle", "rgba(100,150,100,1)")
	ctx.Set("lineWidth", 2*worldScale)

	renderFrame = js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		now := args[0].Float()
		tdiff := now - tmark
		doc.Call("getElementById", "fps").Set("innerHTML", fmt.Sprintf("FPS: %.01f", 1000/tdiff))
		tmark = now

		// Pool window size to handle resize
		curBodyW := doc.Get("body").Get("clientWidth").Float()
		curBodyH := doc.Get("body").Get("clientHeight").Float()
		if curBodyW != width || curBodyH != height {
			width, height = curBodyW, curBodyH
			canvasEl.Set("width", width)
			canvasEl.Set("height", height)
		}
		world.Step(tdiff/1000*simSpeed, 60, 120)

		ctx.Call("clearRect", 0, 0, width*worldScale, height*worldScale)

		for curBody := world.GetBodyList(); curBody != nil; curBody = curBody.M_next {
			// Only one fixture for now
			ctx.Call("save")
			ft := curBody.M_fixtureList
			switch shape := ft.M_shape.(type) {
			case *box2d.B2PolygonShape: // Box
				// canvas translate
				ctx.Call("translate", curBody.M_xf.P.X, curBody.M_xf.P.Y)
				ctx.Call("rotate", curBody.M_xf.Q.GetAngle())
				ctx.Call("beginPath")
				ctx.Call("moveTo", shape.M_vertices[0].X, shape.M_vertices[0].Y)
				for _, v := range shape.M_vertices[1:shape.M_count] {
					ctx.Call("lineTo", v.X, v.Y)
				}
				ctx.Call("lineTo", shape.M_vertices[0].X, shape.M_vertices[0].Y)
				ctx.Call("fill")
				ctx.Call("stroke")
			case *box2d.B2CircleShape:
				ctx.Call("translate", curBody.M_xf.P.X, curBody.M_xf.P.Y)
				ctx.Call("rotate", curBody.M_xf.Q.GetAngle())
				ctx.Call("beginPath")
				ctx.Call("arc", 0, 0, shape.M_radius, 0, 2*math.Pi)
				ctx.Call("fill")
				ctx.Call("moveTo", 0, 0)
				ctx.Call("lineTo", 0, shape.M_radius)
				ctx.Call("stroke")
			}
			ctx.Call("restore")

		}
		// If we have a verts (mouse shape)
		if verts != nil {
			ctx.Call("save")
			ctx.Call("beginPath")
			ctx.Call("moveTo", verts[0].X, verts[0].Y)
			for _, v := range verts[1:] {
				ctx.Call("lineTo", v.X, v.Y)
			}
			ctx.Call("stroke")

			ctx.Set("lineWidth", 4*worldScale)
			for _, v := range verts { // Draw the clickPoints
				ctx.Call("beginPath")
				ctx.Call("arc", v.X, v.Y, 5*worldScale, 0, math.Pi*2)
				ctx.Call("stroke")
			}
			ctx.Call("restore")
		}
		js.Global().Call("requestAnimationFrame", renderFrame)
		return nil
	})

	// Start running
	js.Global().Call("requestAnimationFrame", renderFrame)

	<-done

}
