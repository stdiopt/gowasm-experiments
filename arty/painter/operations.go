package painter

import (
	"encoding/json"
	"errors"
	"image/color"
)

const (
	opInit = iota + 1
	opLine
	opText
)

// OP Wrapper
type Message struct {
	Payload interface{}
}

func (m *Message) UnmarshalJSON(raw []byte) error {
	v := struct {
		OP      uint
		Payload json.RawMessage
	}{}
	err := json.Unmarshal(raw, &v)
	if err != nil {
		return err
	}
	switch v.OP {
	case opInit:
		payload := InitOP{}
		err = json.Unmarshal(v.Payload, &payload)
		m.Payload = payload
	case opLine:
		payload := LineOP{}
		err = json.Unmarshal(v.Payload, &payload)
		m.Payload = payload
	case opText:
		payload := TextOP{}
		err = json.Unmarshal(v.Payload, &payload)
		m.Payload = payload
	default:
		return errors.New("unknown operation")
	}
	return err
}
func (m Message) MarshalJSON() ([]byte, error) {
	v := struct {
		OP      uint
		Payload interface{}
	}{
		Payload: m.Payload,
	}
	switch m.Payload.(type) {
	case InitOP:
		v.OP = opInit
	case LineOP:
		v.OP = opLine
	case TextOP:
		v.OP = opText
	}
	return json.Marshal(v)
}

// This ops will be marshalled with the wrapper struct
type InitOP struct {
	Width, Height int
	Data          []byte
}

type LineOP struct {
	Color  color.RGBA
	Width  float64
	X1, Y1 float64
	X2, Y2 float64
}

type TextOP struct {
	Color color.RGBA
	Size  float64
	X, Y  float64
	Text  string
}
