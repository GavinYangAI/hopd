package ipc

import (
	"encoding/json"
	"io"
)

// Encoder writes newline-delimited JSON values.
type Encoder struct{ enc *json.Encoder }

// NewEncoder returns an Encoder. encoding/json's Encoder.Encode already
// appends a single newline after each value, giving us JSON-Lines framing.
func NewEncoder(w io.Writer) *Encoder { return &Encoder{enc: json.NewEncoder(w)} }

// Encode writes one value followed by a newline.
func (e *Encoder) Encode(v any) error { return e.enc.Encode(v) }

// Decoder reads newline-delimited JSON values.
type Decoder struct{ dec *json.Decoder }

// NewDecoder returns a Decoder.
func NewDecoder(r io.Reader) *Decoder { return &Decoder{dec: json.NewDecoder(r)} }

// Decode reads the next JSON value into v.
func (d *Decoder) Decode(v any) error { return d.dec.Decode(v) }
