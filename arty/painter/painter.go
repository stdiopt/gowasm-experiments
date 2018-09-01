package painter

import (
	"encoding/json"
	"image"

	"github.com/llgcode/draw2d/draw2dimg"
)

type BufPainter struct {
	image  *image.RGBA
	OnInit func(InitOP)
}

func New() *BufPainter {
	im := image.NewRGBA(image.Rect(0, 0, 1, 1))
	return &BufPainter{im, nil}
}

func (p *BufPainter) HandleRaw(msg []byte) error {
	m := OP{}
	err := json.Unmarshal(msg, &m)
	if err != nil {
		return err
	}

	return p.Handle(m)

}

func (p *BufPainter) Handle(m OP) error {
	switch m.OP {
	case OPInit:
		im := InitOP{}
		err := json.Unmarshal(m.Payload, &im)
		if err != nil {
			return err
		}
		p.Init(im)

	case OPLine:
		lm := LineOP{}
		err := json.Unmarshal(m.Payload, &lm)
		if err != nil {
			return err
		}
		p.Line(lm)
	}
	return nil
}

func (p *BufPainter) Init(op InitOP) {
	p.image = image.NewRGBA(image.Rect(0, 0, op.Width, op.Height))
	p.Set(op.Data)
	if p.OnInit != nil {
		p.OnInit(op)
	}
}

// We could have more ops in the future
func (p *BufPainter) Line(op LineOP) {
	c := draw2dimg.NewGraphicContext(p.image)
	c.SetStrokeColor(op.Color)
	c.SetLineWidth(op.Width)
	c.BeginPath()
	c.MoveTo(op.X1, op.Y1)
	c.LineTo(op.X2, op.Y2)
	c.Stroke()
}

func (p *BufPainter) Set(buf []byte) {
	if buf == nil {
		return
	}
	copy(p.image.Pix, buf)
}

func (p *BufPainter) ImageData() []byte {
	return p.image.Pix
}

func (p *BufPainter) Width() int {
	return p.image.Bounds().Max.X
}

func (p *BufPainter) Height() int {
	return p.image.Bounds().Max.Y
}
