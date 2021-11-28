package types

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

const (
	BinaryType uint8 = iota + 1
	StringType
	MaxPayloadSize uint32 = 10 << 20 // 10 MB
)

var ErrMaxPayloadSize = errors.New("maximum payload size exceeded")

// Payload is implemented by any value that has a
// Bytes, String, ReadFrom and WriteTo methods.
type Payload interface {
	fmt.Stringer
	io.ReaderFrom
	io.WriterTo
	Bytes() []byte
}

// Binary is the struct for binary payloads that follow the TLV (type-length-value) pattern
type Binary []byte

func (b Binary) Bytes() []byte { return b }

func (b Binary) String() string { return string(b) }

func (b Binary) WriteTo(w io.Writer) (int64, error) {
	// first writes the 1-byte type to the writer
	err := binary.Write(w, binary.BigEndian, BinaryType) // 1-byte type
	if err != nil {
		return 0, err
	}
	var n int64 = 1

	// then writes the 4-byte length of the Binary to the writer
	err = binary.Write(w, binary.BigEndian, uint32(len(b))) // 4-byte size
	if err != nil {
		return n, err
	}

	n += 4

	// Write the Binary value itself to the writer
	o, err := w.Write(b) // payload

	return n + int64(o), err
}

func (b *Binary) ReadFrom(r io.Reader) (int64, error) {
	var typ uint8
	// read 1 byte from the reader into typ
	err := binary.Read(r, binary.BigEndian, &typ) // 1-byte type
	if err != nil {
		return 0, err
	}

	var n int64 = 1
	// verify BinaryType
	if typ != BinaryType {
		return n, errors.New("invalid Binary")
	}

	var size uint32
	// read 4 bytes into the size variable
	err = binary.Read(r, binary.BigEndian, &size) // 4-byte size
	if err != nil {
		return n, err
	}

	n += 4

	if size > MaxPayloadSize {
		return n, ErrMaxPayloadSize
	}

	// set size of new Binary byte slice
	*b = make([]byte, size)

	// populate the Binary byte slice with payload
	o, err := r.Read(*b)

	return n + int64(o), err
}

// String is the struct for string payloads that follow the TLV (type-length-value) pattern
type String string

func (s String) Bytes() []byte { return []byte(s) }

func (s String) String() string { return string(s) }

func (s String) WriteTo(w io.Writer) (int64, error) {
	// first writes the 1-byte type to the writer
	err := binary.Write(w, binary.BigEndian, StringType) // 1-byte type
	if err != nil {
		return 0, err
	}
	var n int64 = 1

	// then writes the 4-byte length of the String to the writer
	err = binary.Write(w, binary.BigEndian, uint32(len(s))) // 4-byte size
	if err != nil {
		return n, err
	}

	n += 4

	// Write the String value itself to the writer
	o, err := w.Write([]byte(s)) // payload

	return n + int64(o), err
}

func (s *String) ReadFrom(r io.Reader) (int64, error) {
	var typ uint8
	// read 1 byte from the reader into typ
	err := binary.Read(r, binary.BigEndian, &typ) // 1-byte type
	if err != nil {
		return 0, err
	}

	var n int64 = 1
	// verify StringType
	if typ != StringType {
		return n, errors.New("invalid String")
	}

	var size uint32
	// read 4 bytes into the size variable
	err = binary.Read(r, binary.BigEndian, &size) // 4-byte size
	if err != nil {
		return n, err
	}

	n += 4

	buf := make([]byte, size)
	o, err := r.Read(buf) // payload
	if err != nil {
		return n, err
	}

	// casts the value read from the reader to a String
	*s = String(buf)

	return n + int64(o), nil
}

func decode(r io.Reader) (Payload, error) {
	var typ uint8
	err := binary.Read(r, binary.BigEndian, &typ)
	if err != nil {
		return nil, err
	}

	var payload Payload

	switch typ {
	case BinaryType:
		payload = new(Binary)
	case StringType:
		payload = new(String)
	default:
		return nil, errors.New("unknown type")
	}

	_, err = payload.ReadFrom(
		io.MultiReader(bytes.NewReader([]byte{typ}), r))
	if err != nil {
		return nil, err
	}

	return payload, nil
}
