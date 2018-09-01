package painter

import (
	"encoding/json"
	"image/color"
)

const (
	OPInit = iota + 1
	OPLine
	OpClear
)

type OP struct {
	OP      uint
	Payload json.RawMessage
}

// This ops will be marshalled with the wrapper struct

type InitOP struct {
	Width, Height int
	Data          []byte
}

func (o InitOP) MarshalJSON() ([]byte, error) {
	type alias InitOP // this will prevent recursion on marshal
	payload, err := json.Marshal(alias(o))
	if err != nil {
		return nil, err
	}
	return json.Marshal(OP{OPInit, payload})
}

type LineOP struct {
	Color  color.RGBA
	Width  float64
	X1, Y1 float64
	X2, Y2 float64
}

func (o LineOP) MarshalJSON() ([]byte, error) {
	type alias LineOP
	payload, err := json.Marshal(alias(o))
	if err != nil {
		return nil, err
	}
	return json.Marshal(OP{OPLine, payload})
}
