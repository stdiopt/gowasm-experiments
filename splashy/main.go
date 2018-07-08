//Wasming
// compile: GOOS=js GOARCH=wasm go build -o main.wasm ./main.go
package main

import (
	"fmt"
	"log"
	"math/rand"
	"strconv"

	"syscall/js"
	// this box2d throws some unexpected panics
	"github.com/ByteArena/box2d"
)

var (
	width      int
	height     int
	ctx        js.Value
	simSpeed   = 1.0
	worldScale = 0.0125 // 1/8
	resDiv     = 8
	maxBodies  = 150
)

func main() {

	// Init Canvas stuff
	doc := js.Global().Get("document")
	canvasEl := doc.Call("getElementById", "mycanvas")
	width = doc.Get("body").Get("clientWidth").Int()
	height = doc.Get("body").Get("clientHeight").Int()
	canvasEl.Set("width", width)
	canvasEl.Set("height", height)

	gl := canvasEl.Call("getContext", "webgl")
	if gl == js.Undefined() {
		gl = canvasEl.Call("getContext", "experimental-webgl")
	}
	// once again
	if gl == js.Undefined() {
		js.Global().Call("alert", "browser might not support webgl")
		return
	}

	done := make(chan struct{}, 0)

	thing := Thing{}
	mouseDown := false
	// Handle mouse
	doc.Call("addEventListener", "mousedown", js.NewCallback(func(args []js.Value) {
		mouseDown = true
		evt := args[0]
		if evt.Get("target") != canvasEl {
			return
		}
		mx := evt.Get("clientX").Float() * worldScale
		my := evt.Get("clientY").Float() * worldScale
		thing.AddCircle(mx, my)

	}))
	doc.Call("addEventListener", "mouseup", js.NewCallback(func(args []js.Value) {
		mouseDown = false
	}))

	doc.Call("addEventListener", "mousemove", js.NewCallback(func(args []js.Value) {
		if !mouseDown {
			return
		}
		evt := args[0]
		if evt.Get("target") != canvasEl {
			return
		}
		mx := evt.Get("clientX").Float() * worldScale
		my := evt.Get("clientY").Float() * worldScale
		thing.AddCircle(mx, my)

		// Close at some point create an object and set to nil
	}))
	// Speed control input
	doc.Call("getElementById", "speed").Call("addEventListener", "input", js.NewCallback(func(args []js.Value) {
		evt := args[0]
		fval, err := strconv.ParseFloat(evt.Get("target").Get("value").String(), 64)
		if err != nil {
			log.Println("Invalid value", err)
			return
		}
		simSpeed = fval
	}))

	err := thing.Init(gl)
	if err != nil {
		js.Global().Call("alert", err.Error())
		return
	}

	// Draw things
	var renderFrame js.Callback
	var tmark float64
	var markCount = 0
	var tdiffSum float64

	renderFrame = js.NewCallback(func(args []js.Value) {
		// Update the DOM less frequently
		now := args[0].Float()
		tdiff := now - tmark
		tdiffSum += tdiff
		markCount++
		if markCount > 10 {
			doc.Call("getElementById", "fps").Set("innerHTML", fmt.Sprintf("FPS: %.01f", 1000/(tdiffSum/float64(markCount))))
			tdiffSum, markCount = 0, 0
		}
		tmark = now
		// --
		thing.Render(gl, tdiff/1000)

		js.Global().Call("requestAnimationFrame", renderFrame)
	})

	// Start running
	js.Global().Call("requestAnimationFrame", renderFrame)

	<-done

}

type Thing struct {
	// dot shaders
	prog        js.Value
	aPosition   js.Value
	uFragColor  js.Value
	uResolution js.Value

	dotBuf     js.Value
	qBlur      *QuadFX
	qThreshold *QuadFX
	rtTex      [2]js.Value // render target Texture
	rt         [2]js.Value // framebuffer(render target)

	rdotBuf []float32
	world   box2d.B2World
}

func (t *Thing) Init(gl js.Value) error {
	// Drawing program
	var err error
	t.prog, err = programFromSrc(gl, dotVertShader, dotFragShader)
	if err != nil {
		return err
	}
	t.aPosition = gl.Call("getAttribLocation", t.prog, "a_position")
	t.uFragColor = gl.Call("getUniformLocation", t.prog, "uFragColor")
	t.uResolution = gl.Call("getUniformLocation", t.prog, "uResolution")

	t.dotBuf = gl.Call("createBuffer", gl.Get("ARRAY_BUFFER"))
	//renderer targets
	for i := 0; i < 2; i++ {
		t.rtTex[i] = createTexture(gl, width/resDiv, height/resDiv)
		t.rt[i] = createFB(gl, t.rtTex[i])
	}

	t.qBlur = &QuadFX{}
	err = t.qBlur.Init(gl, blurShader)
	if err != nil {
		log.Fatal("Error:", err)
	}
	t.qThreshold = &QuadFX{}
	err = t.qThreshold.Init(gl, thresholdShader)
	if err != nil {
		log.Fatal("Error:", err)
	}

	//////////////////////////
	// Physics
	// ///////////
	t.world = box2d.MakeB2World(box2d.B2Vec2{X: 0, Y: 9.8})
	floor := t.world.CreateBody(&box2d.B2BodyDef{
		Type:     box2d.B2BodyType.B2_kinematicBody,
		Position: box2d.B2Vec2{X: 0, Y: float64(height+10) * worldScale},
		Active:   true,
	})
	floorShape := &box2d.B2PolygonShape{}
	floorShape.SetAsBox(float64(width)*worldScale, 20*worldScale)
	ft := floor.CreateFixture(floorShape, 1)
	ft.M_friction = 0.3

	// Walls
	wallShape := &box2d.B2PolygonShape{}
	wallShape.SetAsBox(20*worldScale, float64(height)*worldScale)

	wallL := t.world.CreateBody(&box2d.B2BodyDef{
		Type:     box2d.B2BodyType.B2_kinematicBody,
		Position: box2d.B2Vec2{X: 0, Y: 0},
		Active:   true,
	})
	wlf := wallL.CreateFixture(wallShape, 1)
	wlf.M_friction = 0.3

	wallR := t.world.CreateBody(&box2d.B2BodyDef{
		Type:     box2d.B2BodyType.B2_kinematicBody,
		Position: box2d.B2Vec2{X: float64(width) * worldScale, Y: 0},
		Active:   true,
	})
	wrt := wallR.CreateFixture(wallShape, 1)
	wrt.M_friction = 0.3

	for i := 0; i < 10; i++ {
		t.AddCircle(rand.Float64()*float64(width)*worldScale, rand.Float64()*float64(height)*worldScale)
	}

	t.rdotBuf = make([]float32, (maxBodies+1)*3)

	return nil
}

func (t *Thing) Render(gl js.Value, dtTime float64) {

	t.world.Step(dtTime*simSpeed, 3, 3)
	i := 0
	count := 0
	for curBody := t.world.GetBodyList(); curBody != nil; curBody = curBody.M_next {
		ft := curBody.M_fixtureList
		switch shape := ft.M_shape.(type) {
		case *box2d.B2CircleShape: // Box
			t.rdotBuf[i] = float32(curBody.M_xf.P.X / (float64(width) * worldScale))
			t.rdotBuf[i+1] = float32(curBody.M_xf.P.Y / (float64(height) * worldScale))
			t.rdotBuf[i+2] = float32(shape.M_radius)
			i += 3
		}
		count++
		// Stop processing
		if count > maxBodies {
			break
		}
	}

	// Render Dots to texture first
	//
	gl.Call("bindFramebuffer", gl.Get("FRAMEBUFFER"), t.rt[0])
	gl.Call("viewport", 0, 0, width/resDiv, height/resDiv) //texSize

	gl.Call("clearColor", 0, 0, 0, 0)
	gl.Call("clear", gl.Get("COLOR_BUFFER_BIT"))

	// DotRenderer
	gl.Call("useProgram", t.prog)
	gl.Call("bindBuffer", gl.Get("ARRAY_BUFFER"), t.dotBuf)

	ta := js.TypedArrayOf(t.rdotBuf)
	gl.Call("bufferData", gl.Get("ARRAY_BUFFER"), ta, gl.Get("STATIC_DRAW")) // upload to gpu
	ta.Release()
	gl.Call("enableVertexAttribArray", t.aPosition)
	gl.Call("vertexAttribPointer", t.aPosition, 3, gl.Get("FLOAT"), false, 0, 0)
	gl.Call("uniform4f", t.uFragColor, 1.0, 0.0, 0.0, 1.0)
	gl.Call("uniform2f", t.uResolution, float64(width)*worldScale, float64(height)*worldScale)
	gl.Call("drawArrays", gl.Get("POINTS"), 0, count)
	// /DotRenderer

	/// FX Blurx4 TODO: better blur
	for i := 0; i < 4; i++ {
		gl.Call("bindFramebuffer", gl.Get("FRAMEBUFFER"), t.rt[1])
		gl.Call("viewport", 0, 0, width/resDiv, height/resDiv)
		gl.Call("bindTexture", gl.Get("TEXTURE_2D"), t.rtTex[0])
		t.qBlur.Render(gl)

		gl.Call("bindFramebuffer", gl.Get("FRAMEBUFFER"), t.rt[0])
		gl.Call("viewport", 0, 0, width/resDiv, height/resDiv)
		gl.Call("bindTexture", gl.Get("TEXTURE_2D"), t.rtTex[1])
		t.qBlur.Render(gl)
	}
	/// FX Threshold to Screen
	gl.Call("bindFramebuffer", gl.Get("FRAMEBUFFER"), nil)
	gl.Call("viewport", 0, 0, width, height)
	gl.Call("bindTexture", gl.Get("TEXTURE_2D"), t.rtTex[0])
	t.qThreshold.Render(gl)

}

func (t *Thing) AddCircle(mx, my float64) {
	if t.world.GetBodyCount() > maxBodies {
		// Check for the last on list and delete backwards:o
		var b *box2d.B2Body
		// theres is no M_last but we could cache it somewhere
		for b = t.world.GetBodyList(); b.M_next != nil; b = b.M_next {
		}
		// Search backwards for a circle (ignoring the walls/floors)
		for ; b != nil; b = b.M_prev {
			if _, ok := b.M_fixtureList.M_shape.(*box2d.B2CircleShape); ok {
				t.world.DestroyBody(b) // Destroy first body
				break
			}
		}
	}
	obj1 := t.world.CreateBody(&box2d.B2BodyDef{
		Type:         box2d.B2BodyType.B2_dynamicBody,
		Position:     box2d.B2Vec2{X: mx, Y: my},
		Awake:        true,
		Active:       true,
		GravityScale: 1.0,
	})
	shape := box2d.NewB2CircleShape()
	shape.M_radius = (10 + rand.Float64()*10) * worldScale
	ft := obj1.CreateFixture(shape, 1)
	ft.M_friction = 0.2
	ft.M_restitution = 0.6
}

//// SHADERS & Utils
const dotVertShader = `
	attribute vec3 a_position;
	void main () {
		gl_Position = vec4(a_position.xy * 2.0 - 1.0, 0, 1);
		gl_PointSize = a_position.z * 30.0;
	}
`
const dotFragShader = `
	precision mediump float;
	uniform vec4 uFragColor;
	void main () {
		vec2 pt = gl_PointCoord - vec2(0.5);
		if(pt.x*pt.x+pt.y*pt.y > 0.25)
		  discard;
		gl_FragColor = uFragColor;
	}
`

const blurShader = `
precision mediump float;
uniform sampler2D u_image;
uniform vec2 u_textureSize;
varying vec2 v_texCoord;
void main() {
	vec2 onePixel = vec2(1,1) / u_textureSize;
	vec4 colorSum =
     texture2D(u_image, v_texCoord + onePixel * vec2(-1, -1)) + 
     texture2D(u_image, v_texCoord + onePixel * vec2( 0, -1)) +
     texture2D(u_image, v_texCoord + onePixel * vec2( 1, -1)) +
     texture2D(u_image, v_texCoord + onePixel * vec2(-1,  0)) +
     texture2D(u_image, v_texCoord + onePixel * vec2( 0,  0)) +
     texture2D(u_image, v_texCoord + onePixel * vec2( 1,  0)) +
     texture2D(u_image, v_texCoord + onePixel * vec2(-1,  1)) +
     texture2D(u_image, v_texCoord + onePixel * vec2( 0,  1)) +
     texture2D(u_image, v_texCoord + onePixel * vec2( 1,  1));
  gl_FragColor = colorSum / 9.0;
}
`
const thresholdShader = `
precision mediump float;
uniform sampler2D u_image;
varying vec2 v_texCoord;
void main() {
	vec4 col = texture2D(u_image,v_texCoord);
	if (col.r < 0.4) discard;
	gl_FragColor = vec4(1.0,0.0,0.0,1.0);
}

`

const vertQuad = `
attribute vec2 a_position;
attribute vec2 a_texCoord;
varying vec2 v_texCoord;
void main() {
   gl_Position = vec4((a_position * 2.0 - 1.0) * vec2(1,-1), 0, 1);
   v_texCoord = a_texCoord;
 }
`

type QuadFX struct {
	prog         js.Value
	aPosition    js.Value
	aTexCoord    js.Value
	uTextureSize js.Value

	quadBuf js.Value
}

func (q *QuadFX) Init(gl js.Value, frag string) error {
	var err error
	q.prog, err = programFromSrc(gl, vertQuad, frag)
	if err != nil {
		return err
	}

	q.aPosition = gl.Call("getAttribLocation", q.prog, "a_position")
	q.aTexCoord = gl.Call("getAttribLocation", q.prog, "a_texCoord")
	q.uTextureSize = gl.Call("getUniformLocation", q.prog, "u_textureSize")

	q.quadBuf = gl.Call("createBuffer")
	// texCoord/posCoord
	gl.Call("bindBuffer", gl.Get("ARRAY_BUFFER"), q.quadBuf)
	gl.Call("bufferData", gl.Get("ARRAY_BUFFER"), js.TypedArrayOf([]float32{
		0.0, 0.0, 1.0, 0.0, 0.0, 1.0,
		0.0, 1.0, 1.0, 0.0, 1.0, 1.0,
	}), gl.Get("STATIC_DRAW"))
	return nil

}
func (q *QuadFX) Render(gl js.Value) {
	gl.Call("useProgram", q.prog)
	// Vertex
	gl.Call("bindBuffer", gl.Get("ARRAY_BUFFER"), q.quadBuf)

	gl.Call("enableVertexAttribArray", q.aPosition)
	gl.Call("vertexAttribPointer", q.aPosition, 2, gl.Get("FLOAT"), false, 0, 0)
	gl.Call("enableVertexAttribArray", q.aTexCoord) // sabe buf
	gl.Call("vertexAttribPointer", q.aTexCoord, 2, gl.Get("FLOAT"), false, 0, 0)

	gl.Call("uniform2f", q.uTextureSize, width/resDiv, height/resDiv)

	gl.Call("drawArrays", gl.Get("TRIANGLES"), 0 /*offset*/, 6 /*count*/)

}

// Helper funcs

// Render to framebuffer first, then framebuffer to screen
func compileShader(gl, shaderType js.Value, shaderSrc string) (js.Value, error) {
	var shader = gl.Call("createShader", shaderType)
	gl.Call("shaderSource", shader, shaderSrc)
	gl.Call("compileShader", shader)

	if !gl.Call("getShaderParameter", shader, gl.Get("COMPILE_STATUS")).Bool() {
		return js.Undefined(), fmt.Errorf("could not compile shader: %v", gl.Call("getShaderInfoLog", shader).String())
	}
	return shader, nil
}

func linkProgram(gl, vertexShader, fragmentShader js.Value) (js.Value, error) {
	var program = gl.Call("createProgram")
	gl.Call("attachShader", program, vertexShader)
	gl.Call("attachShader", program, fragmentShader)
	gl.Call("linkProgram", program)
	if !gl.Call("getProgramParameter", program, gl.Get("LINK_STATUS")).Bool() {
		return js.Undefined(), fmt.Errorf("could not link program: %v", gl.Call("getProgramInfoLog", program).String())
	}

	return program, nil
}

func programFromSrc(gl js.Value, vertShaderSrc, fragShaderSrc string) (js.Value, error) {
	vertexShader, err := compileShader(gl, gl.Get("VERTEX_SHADER"), vertShaderSrc)
	if err != nil {
		return js.Undefined(), err
	}
	fragShader, err := compileShader(gl, gl.Get("FRAGMENT_SHADER"), fragShaderSrc)
	if err != nil {
		return js.Undefined(), err
	}
	prog, err := linkProgram(gl, vertexShader, fragShader)
	if err != nil {
		return js.Undefined(), err
	}
	return prog, nil
}

func createTexture(gl js.Value, width, height int) js.Value {
	tex := gl.Call("createTexture")
	gl.Call("bindTexture", gl.Get("TEXTURE_2D"), tex)
	// define size and format of level 0
	gl.Call("texImage2D",
		gl.Get("TEXTURE_2D"),
		0,
		gl.Get("RGBA"),
		width, height,
		0,
		gl.Get("RGBA"),
		gl.Get("UNSIGNED_BYTE"),
		js.Null(),
	)

	// set the filtering so we don't need mips
	gl.Call("texParameteri", gl.Get("TEXTURE_2D"), gl.Get("TEXTURE_MIN_FILTER"), gl.Get("LINEAR"))
	gl.Call("texParameteri", gl.Get("TEXTURE_2D"), gl.Get("TEXTURE_MAG_FILTER"), gl.Get("LINEAR"))
	gl.Call("texParameteri", gl.Get("TEXTURE_2D"), gl.Get("TEXTURE_WRAP_S"), gl.Get("CLAMP_TO_EDGE"))
	gl.Call("texParameteri", gl.Get("TEXTURE_2D"), gl.Get("TEXTURE_WRAP_T"), gl.Get("CLAMP_TO_EDGE"))

	return tex
}

// Create framebuffer binded to texture
func createFB(gl, tex js.Value) js.Value {
	fb := gl.Call("createFramebuffer")
	gl.Call("bindFramebuffer", gl.Get("FRAMEBUFFER"), fb)
	gl.Call("framebufferTexture2D", gl.Get("FRAMEBUFFER"), gl.Get("COLOR_ATTACHMENT0"), gl.Get("TEXTURE_2D"), tex, 0)
	return fb
}
