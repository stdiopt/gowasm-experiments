// +build js,wasm

package main

import (
	"encoding/base64"
	"fmt"
	"math/rand"
	"strconv"
	"syscall/js"
	"time"
)

func main() {
	t := &audioThing{}
	t.Start()
}

type dom struct {
	doc     js.Value
	beat    js.Value
	bpm     js.Value
	bpmLbl  js.Value
	tlen    js.Value
	tlenLbl js.Value
}

type audioThing struct {
	ctx js.Value
	el  dom
	//beatEl js.Value

	ins     []func()
	seq     []bool
	curStep int

	BPM      byte
	trackLen byte

	playing bool
	done    chan struct{}

	wn js.Value
}

func (t *audioThing) Start() {

	t.done = make(chan struct{}, 0)

	doc := js.Global().Get("document")
	actx := js.Global().Get("AudioContext")
	if !actx.Truthy() {
		actx = js.Global().Get("webkitAudioContext") // safari
	}

	t.ctx = actx.New()

	t.el.beat = doc.Call("getElementById", "beat")
	t.el.bpm = doc.Call("getElementById", "bpm")
	t.el.bpmLbl = t.el.bpm.Get("nextElementSibling")
	t.el.tlen = doc.Call("getElementById", "tlen")
	t.el.tlenLbl = t.el.tlen.Get("nextElementSibling")

	notes := []float64{
		523.3, 554.4, 587.3, 622.3, 659.3, 698.5,
		740.0, 784.0, 830.6, 880.0, 932.3, 987.8,
	}
	// Initialize instruments
	t.ins = []func(){
		t.playKick,
		t.playSnare,
		t.playCHithat,
	}
	// Add tune
	for i := 0; i <= 12; i++ {
		n := notes[i%len(notes)] * (1 + float64((i / len(notes))))
		t.ins = append(t.ins, t.createTune(n))
	}
	bufSize := 4096
	t.wn = t.ctx.Call("createScriptProcessor", bufSize, 1, 1)
	whiteNoiseFn := js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		e := args[0]
		bufSize := 4096
		out := e.Get("outputBuffer").Call("getChannelData", 0)
		for i := 0; i < bufSize; i++ {
			out.SetIndex(i, rand.Float64()*2-1)
		}
		return nil
	})
	defer whiteNoiseFn.Release()
	t.wn.Set("onaudioprocess", whiteNoiseFn)

	t.setBPM(80)
	t.setTrackLen(32)

	go t.handleEvents()
	t.hashRestore()

	<-t.done
}

func (t *audioThing) play() {
	if t.playing {
		return
	}
	t.playing = true

	go func() {
		for {
			beat := time.After(time.Minute / time.Duration(t.BPM) / 4)
			select {
			case <-t.done:
				return
			case <-beat:
				t.step()
			}
		}
	}()
}

// Build beats DOM
func (t *audioThing) buildDOM() {
	beatHTML := ""
	for i := 0; i < len(t.seq)/len(t.ins); i++ {
		stepHTML := ""
		for j := 0; j < len(t.ins); j++ {
			stepHTML += fmt.Sprintf(
				`<div class="key" key="%d"></div>`,
				i*len(t.ins)+j,
			)
		}
		beatHTML += fmt.Sprintf(`<div class="step">%s</div>`, stepHTML)
	}
	t.el.beat.Set("innerHTML", beatHTML)
}

func (t *audioThing) setTrackLen(n byte) {
	t.curStep = 0
	t.trackLen = n
	t.el.tlenLbl.Set("innerHTML", fmt.Sprint(n))
	t.el.tlen.Set("value", fmt.Sprint(n))

	t.seq = make([]bool, int(n)*len(t.ins))
	t.buildDOM()
}
func (t *audioThing) setBPM(n byte) {
	t.BPM = n
	t.el.bpmLbl.Set("innerHTML", fmt.Sprintf("%d bpm", n))
	t.el.bpm.Set("value", n)
}

// Set DOM events
func (t *audioThing) handleEvents() {
	// Handle document click
	handleClick := js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		ev := args[0]
		target := ev.Get("target")
		if target.Call("matches", "#play").Bool() {
			t.play()
			target.Call("setAttribute", "disabled", "disabled")
			return nil
		}
		if !target.Call("matches", ".key").Bool() {
			return nil
		}
		keyIs := ev.Get("target").Call("getAttribute", "key").String()
		keyI, err := strconv.Atoi(keyIs)
		if err != nil {
			println("wrong key", keyIs)
			return nil
		}
		t.seq[keyI] = !t.seq[keyI]
		ev.Get("target").Get("classList").Call("toggle", "active", t.seq[keyI])
		t.hashStore()
		return nil
	})
	defer handleClick.Release()
	js.Global().Call("addEventListener", "click", handleClick)

	// handle location hashChange
	handleHashChange := js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		t.hashRestore()
		return nil
	})
	defer handleHashChange.Release()
	js.Global().Call("addEventListener", "hashchange", handleHashChange)

	// handle Bpm input range
	handleBpmInput := js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		e := args[0]
		sval := e.Get("target").Get("value").String()
		bpm, err := strconv.Atoi(sval)
		if err != nil {
			println("wrong input value")
			return nil
		}

		t.setBPM(byte(bpm))
		t.hashStore()
		return nil
	})
	defer handleBpmInput.Release()
	t.el.bpm.Call("addEventListener", "input", handleBpmInput)

	handleTrackLenInput := js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		e := args[0]
		sval := e.Get("target").Get("value").String()
		tlen, err := strconv.Atoi(sval)
		if err != nil {
			println("wrong input value")
			return nil
		}
		t.setTrackLen(byte(tlen))
		t.hashStore()
		return nil
	})
	defer handleTrackLenInput.Release()
	t.el.tlen.Call("addEventListener", "input", handleTrackLenInput)

	<-t.done
}

func (t *audioThing) step() {
	t.el.beat.
		Get("children").
		Index(t.curStep).
		Get("classList").
		Call("toggle", "current", false)
	t.curStep = (t.curStep + 1) % int(t.trackLen)
	t.el.beat.
		Get("children").
		Index(t.curStep).
		Get("classList").
		Call("toggle", "current", true)
	for i := range t.ins {
		if t.seq[t.curStep*len(t.ins)+i] {
			t.ins[i]()
		}
	}
}

func (t *audioThing) hashStore() {
	var bitbuf = make([]byte, len(t.seq)/8+3)
	bitbuf[0] = t.BPM
	bitbuf[1] = t.trackLen
	bcur := 1
	for i := range t.seq {
		if i&7 == 0 {
			bcur++
		}
		v := 0
		if t.seq[i] {
			v = 1
		}
		bitbuf[bcur] |= byte(v << uint(7-(i&7)))
	}
	b64 := base64.StdEncoding.EncodeToString(bitbuf)
	js.Global().Get("history").Call("pushState", b64, "", "#"+b64)
}

func (t *audioThing) hashRestore() {
	hash := js.Global().Get("location").Get("hash").String()
	if len(hash) == 0 {
		return
	}
	hash = hash[1:] // skip '#'
	bitbuf, err := base64.StdEncoding.DecodeString(hash)
	if err != nil {
		fmt.Println("wrong hash", err)
		return
	}
	t.setBPM(bitbuf[0])
	if bitbuf[1] != t.trackLen {
		t.setTrackLen(bitbuf[1])
	}
	bitbuf = bitbuf[2:]
	steps := []bool{}

	doc := js.Global().Get("document")
	for i := 0; i < len(bitbuf); i++ {
		for j := 7; j >= 0; j-- {
			v := ((bitbuf[i] >> uint(j)) & 1) != 0
			steps = append(steps, v)
			// Set the thing
			el := doc.Call("querySelector",
				fmt.Sprintf(`[key="%d"]`, len(steps)-1),
			)
			if !el.Truthy() {
				continue
			}
			el.Get("classList").Call("toggle", "active", v)
		}
	}
	t.seq = steps
}

// Improve this
func (t *audioThing) playKick() {
	currentTime := t.ctx.Get("currentTime").Float()

	g := t.ctx.Call("createGain")
	g.Call("connect", t.ctx.Get("destination"))
	g.Get("gain").Set("value", 0.4)
	g.Get("gain").Call("linearRampToValueAtTime", 0, currentTime+0.05)

	f := t.ctx.Call("createBiquadFilter")
	f.Call("connect", g)
	f.Set("type", "lowpass")
	f.Get("frequency").Set("value", 400)
	f.Get("frequency").Call("linearRampToValueAtTime", 1, currentTime+0.1)

	o := t.ctx.Call("createOscillator")
	o.Call("connect", g)
	o.Set("type", "sine")
	o.Get("frequency").Set("value", 40)

	var ended js.Func
	ended = js.FuncOf(func(t js.Value, args []js.Value) interface{} {
		g.Call("disconnect")
		ended.Release()
		return nil
	})
	o.Set("onended", ended)
	o.Call("start")
	o.Call("stop", currentTime+0.05)
}

func (t *audioThing) playSnare() {
	currentTime := t.ctx.Get("currentTime").Float()

	g := t.ctx.Call("createGain")
	g.Call("connect", t.ctx.Get("destination"))
	g.Get("gain").Set("value", 0.3)
	g.Get("gain").Call("linearRampToValueAtTime", 0, currentTime+0.1)

	f := t.ctx.Call("createBiquadFilter")
	f.Call("connect", g)
	f.Set("type", "bandpass")
	f.Get("frequency").Set("value", 500)
	f.Get("frequency").Call("linearRampToValueAtTime", 1500, currentTime+0.02)

	o := t.ctx.Call("createOscillator")
	o.Call("connect", f)
	o.Set("type", "sine")
	o.Get("frequency").Set("value", 220)
	o.Get("frequency").Call("linearRampToValueAtTime", 10, currentTime+0.1)

	var ended js.Func
	ended = js.FuncOf(func(t js.Value, args []js.Value) interface{} {
		g.Call("disconnect")
		ended.Release()
		return nil
	})
	o.Set("onended", ended)
	o.Call("start")
	o.Call("stop", currentTime+0.1)

	g2 := t.ctx.Call("createGain")
	g2.Call("connect", f)
	g2.Get("gain").Set("value", 0.1)
	g2.Get("gain").Call("linearRampToValueAtTime", 0.2, currentTime+0.001)

	t.wn.Call("connect", g2)

}

func (t *audioThing) playCHithat() {
	currentTime := t.ctx.Get("currentTime").Float()
	g := t.ctx.Call("createGain")
	g.Call("connect", t.ctx.Get("destination"))
	g.Get("gain").Set("value", 0.01)
	g.Get("gain").Call("linearRampToValueAtTime", 0, currentTime+0.02)

	f := t.ctx.Call("createBiquadFilter")
	f.Call("connect", g)
	f.Set("type", "highpass")
	f.Get("frequency").Set("value", 5000)

	o := t.ctx.Call("createOscillator")
	o.Call("connect", f)
	var ended js.Func
	ended = js.FuncOf(func(t js.Value, args []js.Value) interface{} {
		g.Call("disconnect")
		ended.Release()
		return nil
	})
	o.Set("onended", ended)
	o.Call("start")
	o.Call("stop", currentTime+0.1)
	t.wn.Call("connect", f)

}

// to use lstFreq per instrument
func (t *audioThing) createTune(freq float64) func() {
	return func() {
		currentTime := t.ctx.Get("currentTime").Float()
		g := t.ctx.Call("createGain")
		g.Call("connect", t.ctx.Get("destination"))
		g.Get("gain").Set("value", 0.03)
		g.Get("gain").Call("exponentialRampToValueAtTime", 0.0001, currentTime+2)

		o := t.ctx.Call("createOscillator")
		o.Set("type", "sine")
		o.Get("frequency").Call("linearRampToValueAtTime", freq, currentTime+0.02)
		o.Call("connect", g)

		var ended js.Func
		ended = js.FuncOf(func(t js.Value, args []js.Value) interface{} {
			g.Call("disconnect")
			ended.Release()
			return nil
		})
		o.Set("onended", ended)
		o.Call("start")
		o.Call("stop", currentTime+2)
	}
}
