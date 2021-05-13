// Copyright (c) 2021 Uber Technologies, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

// Package protocol provides implementations of different Thrift protocols.
package protocol

import (
	"io"

	"go.uber.org/thriftrw/wire"
)

// Protocol defines a specific way for a Thrift value to be encoded or
// decoded.
type Protocol interface {
	// Encode the given Value and write the result to the given Writer.
	Encode(v wire.Value, w io.Writer) error

	// EncodeEnveloped encodes the enveloped value and writes the result
	// to the given Writer.
	EncodeEnveloped(e wire.Envelope, w io.Writer) error

	// Decode reads a Value of the given type from the given Reader.
	Decode(r io.ReaderAt, t wire.Type) (wire.Value, error)

	// DecodeEnveloped reads an enveloped value from the given Reader.
	// Enveloped values are assumed to be TStructs.
	DecodeEnveloped(r io.ReaderAt) (wire.Envelope, error)
	StreamWriter(w io.Writer) Writer
}

type Writer interface {
	WriteBool(bool) error
	WriteInt8(i int8) error
	WriteInt16(i int16) error
	WriteInt32(i int32) error
	WriteInt64(i int64) error
	WriteString(s string) error
	WriteDouble(d float64) error
	WriteBinary(b []byte) error
	WriteStructBegin() error
	WriteStructEnd() error
	WriteFieldBegin(wire.Type, int16) error
	WriteFieldEnd() error
	WriteMapBegin(wire.Type, wire.Type, int) error
	WriteMapEnd() error
	WriteSetBegin(wire.Type, int) error
	WriteSetEnd() error
	WriteListBegin(wire.Type, int) error
	WriteListEnd() error
}

type Reader interface {
	ReadBool(int64) (bool, int64, error)
	ReadInt8(int64) (int8, int64, error)
	ReadInt16(int64) (int16, int64, error)
	ReadInt32(int64) (int32, int64, error)
	ReadInt64(int64) (int64, int64, error)
	ReadString(int64) (string, int64, error)
	ReadDouble(int64) (float64, int64, error)
	ReadBinary(int64) ([]byte, int64, error)
	ReadStructBegin(int64) (int64, error)
	ReadStructEnd() error
	ReadFieldBegin(off int64) (wire.Type, int16, int64, error)
	ReadFieldEnd() error
}

type StreamingProtocol interface {
	StreamWriter(io.Writer) Writer
	StreamReader(io.Reader) Reader
}

// EnvelopeAgnosticProtocol defines a specific way for a Thrift value to be
// encoded or decoded, additionally being able to decode requests without prior
// knowledge of whether the request is enveloped.
//
// The Binary protocol in particular can be upcast to EnvelopeAgnosticProtocol.
type EnvelopeAgnosticProtocol interface {
	Protocol

	// DecodeRequest reads an enveloped or un-enveloped struct from the given
	// ReaderAt.
	// This allows a Thrift request handler to transparently accept requests
	// regardless of whether the caller is configured to submit envelopes.
	// The caller specifies the expected envelope type, one of OneWay or Unary,
	// on which the decoder asserts if the envelope is present.
	DecodeRequest(et wire.EnvelopeType, r io.ReaderAt) (wire.Value, Responder, error)
}

// Responder captures how to respond to a request, concerning whether and what
// kind of envelope to use, how to match the sequence identifier of the
// corresponding request.
type Responder interface {
	// EncodeResponse writes a response value to the writer, with the envelope
	// style of the corresponding request.
	// The EnvelopeType should be either wire.Reply or wire.Exception.
	EncodeResponse(v wire.Value, t wire.EnvelopeType, w io.Writer) error
}
