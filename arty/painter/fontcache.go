package painter

import (
	"github.com/golang/freetype/truetype"
	"github.com/llgcode/draw2d"
)

type FontCache map[string]*truetype.Font

func (f FontCache) Load(fd draw2d.FontData) (*truetype.Font, error) {
	font, ok := f[fd.Name]
	if !ok {
		return f["roboto"], nil
	}
	return font, nil
}

func (f *FontCache) Store(fd draw2d.FontData, tf *truetype.Font) {
	(*f)[fd.Name] = tf
}
