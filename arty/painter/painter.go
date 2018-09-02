//go:generate go get github.com/gohxs/folder2go
//go:generate folder2go -nobackup assets font

package painter

import (
	"encoding/json"
	"errors"
	"image"

	"github.com/golang/freetype/truetype"
	"github.com/llgcode/draw2d"
	"github.com/llgcode/draw2d/draw2dimg"
	"github.com/stdiopt/gowasm-experiments/arty/painter/font"
)

type BufPainter struct {
	image    *image.RGBA
	ctx      *draw2dimg.GraphicContext
	font     *truetype.Font
	fontData draw2d.FontData
	OnInit   func(InitOP)
}

func New() (*BufPainter, error) {
	font, err := truetype.Parse(font.Data["font.ttf"])
	if err != nil {
		return nil, err
	}
	return &BufPainter{font: font}, nil
}

func (p *BufPainter) HandleRaw(msg []byte) error {
	m := Message{}
	err := json.Unmarshal(msg, &m)
	if err != nil {
		return err
	}
	return p.HandleOP(m.Payload)
}

func (p *BufPainter) HandleOP(op interface{}) error {
	switch o := op.(type) {
	case InitOP:
		p.Init(o)
	case LineOP:
		p.Line(o)
	case TextOP:
		p.Text(o)
	default:
		return errors.New("unknown op")
	}
	return nil
}

func (p *BufPainter) Set(buf []byte) {
	if buf == nil {
		return
	}
	copy(p.image.Pix, buf)
}

func (p *BufPainter) ImageData() []byte {
	if p.image == nil {
		return nil
	}
	return p.image.Pix
}

func (p *BufPainter) Width() int {
	return p.image.Bounds().Max.X
}

func (p *BufPainter) Height() int {
	return p.image.Bounds().Max.Y
}

func (p *BufPainter) Init(op InitOP) {
	p.image = image.NewRGBA(image.Rect(0, 0, op.Width, op.Height))

	// init font
	fontData := draw2d.FontData{
		Name:   "roboto",
		Family: draw2d.FontFamilySans,
		Style:  draw2d.FontStyleNormal,
	}
	fontCache := &FontCache{}
	fontCache.Store(fontData, p.font)

	p.ctx = draw2dimg.NewGraphicContext(p.image)
	p.ctx.FontCache = fontCache

	p.Set(op.Data) // Image
	if p.OnInit != nil {
		p.OnInit(op)
	}
}
func (p *BufPainter) Line(op LineOP) {
	c := p.ctx
	c.SetStrokeColor(op.Color)
	c.SetLineWidth(op.Width)
	c.BeginPath()
	c.MoveTo(op.X1, op.Y1)
	c.LineTo(op.X2, op.Y2)
	c.Stroke()
}
func (p *BufPainter) Text(op TextOP) {
	c := p.ctx
	c.SetFillColor(op.Color)
	c.SetFont(p.font)
	c.SetFontSize(op.Size)
	c.FillStringAt(op.Text, op.X, op.Y)
}
