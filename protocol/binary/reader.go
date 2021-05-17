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

package binary

import (
	"bytes"
	"fmt"
	"io"
	"math"

	"go.uber.org/thriftrw/wire"
)

// Requests for byte slices longer than this will use a dynamically resizing
// buffer.
const bytesAllocThreshold = 1048576 // 1 MB

// Reader implements a parser for the Thrift Binary Protocol based on an
// io.ReaderAt.
type Reader struct {
	reader io.ReaderAt

	buffReader io.Reader

	// This buffer is re-used every time we need a slice of up to 8 bytes.
	buffer [8]byte
}

// NewReader builds a new Reader based on the given io.ReaderAt.
func NewReader(r io.ReaderAt) Reader {
	return Reader{reader: r}
}

// NewReader builds a new Reader based on the given io.ReaderAt.
func NewBuffReader(r io.Reader) Reader {
	return Reader{buffReader: r}
}

// For the reader, we keep track of the read offset manually everywhere so
// that we can implement lazy collections without extra allocations

// fixedWidth returns the encoded size of a value of the given type. If the
// type's width depends on the value, -1 is returned.
func fixedWidth(t wire.Type) int64 {
	switch t {
	case wire.TBool:
		return 1
	case wire.TI8:
		return 1
	case wire.TDouble:
		return 8
	case wire.TI16:
		return 2
	case wire.TI32:
		return 4
	case wire.TI64:
		return 8
	default:
		return -1
	}
}

func (br *Reader) skipStruct(off int64) (int64, error) {
	typ, off, err := br.readByte(off)
	if err != nil {
		return off, err
	}

	for typ != 0 {
		off += 2 // field ID
		off, err = br.skipValue(wire.Type(typ), off)
		if err != nil {
			return off, err
		}

		typ, off, err = br.readByte(off)
		if err != nil {
			return off, err
		}
	}
	return off, err
}

func (br *Reader) skipMap(off int64) (int64, error) {
	ktByte, off, err := br.readByte(off)
	if err != nil {
		return off, err
	}

	vtByte, off, err := br.readByte(off)
	if err != nil {
		return off, err
	}

	kt := wire.Type(ktByte)
	vt := wire.Type(vtByte)

	count, off, err := br.readInt32(off)
	if err != nil {
		return off, err
	}
	if count < 0 {
		return off, decodeErrorf("negative length %d requested for map", count)
	}

	kw := fixedWidth(kt)
	vw := fixedWidth(vt)
	if kw > 0 && vw > 0 {
		// key and value are fixed width. calculate exact offset increase.
		off += int64(count) * (kw + vw)
		return off, err
	}

	for i := int32(0); i < count; i++ {
		off, err = br.skipValue(kt, off)
		if err != nil {
			return off, err
		}

		off, err = br.skipValue(vt, off)
		if err != nil {
			return off, err
		}
	}
	return off, err
}

func (br *Reader) skipList(off int64) (int64, error) {
	vtByte, off, err := br.readByte(off)
	if err != nil {
		return off, err
	}
	vt := wire.Type(vtByte)

	count, off, err := br.readInt32(off)
	if err != nil {
		return off, err
	}
	if count < 0 {
		return off, decodeErrorf("negative length %d requested for collection", count)
	}

	vw := fixedWidth(vt)
	if vw > 0 {
		// value is fixed width. can calculate new offset right away.
		off += int64(count) * vw
		return off, err
	}

	for i := int32(0); i < count; i++ {
		off, err = br.skipValue(vt, off)
		if err != nil {
			return off, err
		}
	}
	return off, err
}

func (br *Reader) skipValue(t wire.Type, off int64) (int64, error) {
	if w := fixedWidth(t); w > 0 {
		return off + w, nil
	}

	switch t {
	case wire.TBinary:
		length, off, err := br.readInt32(off)
		if err != nil {
			return off, err
		}
		if length < 0 {
			return off, decodeErrorf(
				"negative length %d requested for binary value", length,
			)
		}
		off += int64(length)
		return off, err
	case wire.TStruct:
		return br.skipStruct(off)
	case wire.TMap:
		return br.skipMap(off)
	case wire.TSet:
		return br.skipList(off)
	case wire.TList:
		return br.skipList(off)
	default:
		return off, decodeErrorf("unknown ttype %v", t)
	}
}

func (br *Reader) read(bs []byte, off int64) (int64, error) {
	n, err := br.reader.ReadAt(bs, off)
	off += int64(n)
	if err == io.EOF {
		// All EOFs are unexpected for the decoder
		err = io.ErrUnexpectedEOF
	}
	return off, err
}

// copyN copies n bytes starting at offset off into the given Writer.
func (br *Reader) copyN(w io.Writer, off int64, n int64) (int64, error) {
	src := io.NewSectionReader(br.reader, off, n)
	copied, err := io.CopyN(w, src, n)
	off += copied
	if err == io.EOF {
		// All EOFs are unexpected for the decoder
		err = io.ErrUnexpectedEOF
	}
	return off, err
}

// copyN copies n bytes starting at offset off into the given Writer.
func (br *Reader) copyBuffN(w io.Writer, n int64) (error) {
	_, err := io.CopyN(w, br.buffReader, n)
	if err == io.EOF {
		// All EOFs are unexpected for the decoder
		err = io.ErrUnexpectedEOF
	}
	return err
}

func (br *Reader) readByte(off int64) (byte, int64, error) {
	bs := br.buffer[0:1]
	off, err := br.read(bs, off)
	return bs[0], off, err
}

func (br *Reader) readInt16(off int64) (int16, int64, error) {
	bs := br.buffer[0:2]
	off, err := br.read(bs, off)
	return int16(bigEndian.Uint16(bs)), off, err
}

func (br *Reader) readInt32(off int64) (int32, int64, error) {
	bs := br.buffer[0:4]
	off, err := br.read(bs, off)
	return int32(bigEndian.Uint32(bs)), off, err
}

func (br *Reader) readInt64(off int64) (int64, int64, error) {
	bs := br.buffer[0:8]
	off, err := br.read(bs, off)
	return int64(bigEndian.Uint64(bs)), off, err
}

func (br *Reader) readBytes(off int64) ([]byte, int64, error) {
	length, off, err := br.readInt32(off)
	if err != nil {
		return nil, off, err
	}
	if length < 0 {
		return nil, off, decodeErrorf(
			"negative length %d requested for binary value", length,
		)
	}
	if length == 0 {
		return nil, off, nil
	}

	// Use a dynamically resizing buffer for requests larger than
	// bytesAllocThreshold. We don't want bad requests to lock the system up.
	if length > bytesAllocThreshold {
		var buff bytes.Buffer
		off, err = br.copyN(&buff, off, int64(length))
		if err != nil {
			return nil, off, err
		}
		return buff.Bytes(), off, err
	}

	bs := make([]byte, length)
	off, err = br.read(bs, off)
	return bs, off, err
}

func (br *Reader) readString(off int64) (string, int64, error) {
	v, off, err := br.readBytes(off)
	return string(v), off, err
}

func (br *Reader) readFieldList(off int64) (wire.FieldList, int64, error) {
	fieldList := borrowLazyFieldList(br)
	fieldList.reader = br
	fieldList.offset = off
	off, err := br.skipStruct(off)
	return fieldList, off, err
}

func (br *Reader) readMap(off int64) (wire.MapItemList, int64, error) {
	ktByte, off, err := br.readByte(off)
	if err != nil {
		return nil, off, err
	}

	vtByte, off, err := br.readByte(off)
	if err != nil {
		return nil, off, err
	}

	count, off, err := br.readInt32(off)
	if err != nil {
		return nil, off, err
	}
	if count < 0 {
		return nil, off, decodeErrorf("negative length %d requested for map", count)
	}

	kt := wire.Type(ktByte)
	vt := wire.Type(vtByte)

	start := off
	for i := int32(0); i < count; i++ {
		off, err = br.skipValue(kt, off)
		if err != nil {
			return nil, off, err
		}

		off, err = br.skipValue(vt, off)
		if err != nil {
			return nil, off, err
		}
	}

	items := borrowLazyMapItemList()
	items.ktype = kt
	items.vtype = vt
	items.count = count
	items.reader = br
	items.startOffset = start

	return items, off, err
}

func (br *Reader) readSet(off int64) (wire.ValueList, int64, error) {
	typ, off, err := br.readByte(off)
	if err != nil {
		return nil, off, err
	}

	count, off, err := br.readInt32(off)
	if err != nil {
		return nil, off, err
	}
	if count < 0 {
		return nil, off, decodeErrorf("negative length %d requested for set", count)
	}

	start := off
	for i := int32(0); i < count; i++ {
		off, err = br.skipValue(wire.Type(typ), off)
		if err != nil {
			return nil, off, err
		}
	}

	items := borrowLazyValueList()
	items.count = count
	items.typ = wire.Type(typ)
	items.reader = br
	items.startOffset = start

	return items, off, err
}

func (br *Reader) readList(off int64) (wire.ValueList, int64, error) {
	typ, off, err := br.readByte(off)
	if err != nil {
		return nil, off, err
	}

	count, off, err := br.readInt32(off)
	if err != nil {
		return nil, off, err
	}
	if count < 0 {
		return nil, off, decodeErrorf("negative length %d requested for list", count)
	}

	start := off
	for i := int32(0); i < count; i++ {
		off, err = br.skipValue(wire.Type(typ), off)
		if err != nil {
			return nil, off, err
		}
	}

	items := borrowLazyValueList()
	items.count = count
	items.typ = wire.Type(typ)
	items.reader = br
	items.startOffset = start

	return items, off, err
}

// ReadValue reads a value off the given type off the wire starting at the
// given offset.
//
// Returns the Value, the new offset, and an error if there was a decode error.
func (br *Reader) ReadValue(t wire.Type, off int64) (wire.Value, int64, error) {
	switch t {
	case wire.TBool:
		b, off, err := br.readByte(off)
		if err != nil {
			return wire.Value{}, off, err
		}

		if b != 0 && b != 1 {
			return wire.Value{}, off, decodeErrorf(
				"invalid value %q for bool field", b,
			)
		}

		return wire.NewValueBool(b == 1), off, nil

	case wire.TI8:
		b, off, err := br.readByte(off)
		return wire.NewValueI8(int8(b)), off, err

	case wire.TDouble:
		value, off, err := br.readInt64(off)
		d := math.Float64frombits(uint64(value))
		return wire.NewValueDouble(d), off, err

	case wire.TI16:
		n, off, err := br.readInt16(off)
		return wire.NewValueI16(n), off, err

	case wire.TI32:
		n, off, err := br.readInt32(off)
		return wire.NewValueI32(n), off, err

	case wire.TI64:
		n, off, err := br.readInt64(off)
		return wire.NewValueI64(n), off, err

	case wire.TBinary:
		v, off, err := br.readBytes(off)
		return wire.NewValueBinary(v), off, err

	case wire.TStruct:
		fl, off, err := br.readFieldList(off)
		return wire.NewValueFieldList(fl), off, err

	case wire.TMap:
		m, off, err := br.readMap(off)
		return wire.NewValueMap(m), off, err

	case wire.TSet:
		s, off, err := br.readSet(off)
		return wire.NewValueSet(s), off, err

	case wire.TList:
		l, off, err := br.readList(off)
		return wire.NewValueList(l), off, err

	default:
		return wire.Value{}, off, decodeErrorf("unknown ttype %v", t)
	}
}

func (br *Reader) readBuffByte() (byte, error) {
	bs := br.buffer[0:1]
	n, err := br.buffReader.Read(bs)
	if n != 1 {
		return bs[0], err
	}

	return bs[0], nil
}

func (br *Reader) readBuffI16() (int16, error) {
	bs := br.buffer[0:2]
	n, err := br.buffReader.Read(bs)
	if n != 2 {
		return 0, err
	}
	return int16(bigEndian.Uint16(bs)), err
}

func (br *Reader) readBuffI32() (int32, error) {
	bs := br.buffer[0:4]
	n, err := br.buffReader.Read(bs)
	if n != 4 {
		return 0, err
	}
	return int32(bigEndian.Uint32(bs)), err
}

func (br *Reader) readBuffI64() (int64, error) {
	bs := br.buffer[0:8]
	n, err := br.buffReader.Read(bs)
	if n != 8 {
		return 0, err
	}
	return int64(bigEndian.Uint64(bs)), err
}


func (br *Reader) readBuffBytes() ([]byte, error) {
	length, err := br.readBuffI32()
	if length < 0 {
		return nil, decodeErrorf(
			"negative length %d requested for binary value", length,
		)
	}
	if length == 0 {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	// Use a dynamically resizing buffer for requests larger than
	// bytesAllocThreshold. We don't want bad requests to lock the system up.
	if length > bytesAllocThreshold {
		var buff bytes.Buffer
		err = br.copyBuffN(&buff, int64(length))
		if err != nil {
			return nil, err
		}
		return buff.Bytes(), err
	}

	bs := make([]byte, length)
	n, err := br.buffReader.Read(bs)
	if int32(n) != length {
		return nil, err
	}
	return bs, nil
}

func (br *Reader) ReadBool() (bool, error) {
	b, err := br.readBuffByte()
	if err != nil {
		return false, err
	}
	if b != 0 && b != 1 {
		return false, fmt.Errorf("error in bool")
	}

	return b == 1, nil
}

func (br *Reader) ReadInt8() (int8, error) {
	b, err := br.readBuffByte()
	return int8(b), err
}

func (br *Reader) ReadInt16() (int16, error) {
	return br.readBuffI16()
}

func (br *Reader) ReadInt32() (int32, error) {
	return br.readBuffI32()
}

func (br *Reader) ReadInt64() (int64, error) {
	return br.readBuffI64()
}

func (br *Reader) ReadString() (string, error) {
	//return br.readString(off)
	b, err := br.readBuffBytes()
	if err != nil {
		return "", err
	}

	return string(b), nil
}

func (br *Reader) ReadDouble() (float64, error) {
	val, err := br.readBuffI64()
	if err != nil {
		return 0, err
	}
	return math.Float64frombits(uint64(val)), err
}

func (br *Reader) ReadBinary() ([]byte, error) {
	return br.readBuffBytes()
}

func (br *Reader) ReadStructBegin() (error) {
	return nil
}

func (br *Reader) ReadStructEnd() error {
	//_, err := br.readBuffByte()
	//return err
	return nil
}

func (br *Reader) ReadFieldBegin() (wire.Type, int16, error) {

	ttype, err := br.readBuffByte()
	if err != nil {
		return wire.Type(0), 0, err
	}

	if ttype == 0 {
		return wire.Type(0), 0, nil
	}

	id, err := br.readBuffI16()
	if err != nil {
		return wire.Type(0), 0, err
	}

	return wire.Type(ttype), id, nil
}

func (br *Reader) ReadFieldEnd() error {
	return nil
}

func (br *Reader) ReadListBegin() (wire.Type, int32, error) {
	ttype, err := br.readBuffByte()
	if err != nil {
		return wire.Type(0), 0, err
	}

	length, err := br.readBuffI32()
	if err != nil {
		return wire.Type(0), 0, err
	}

	return wire.Type(ttype), length, nil
}


func (br *Reader) ReadListEnd() error {
	return nil
}

func (br *Reader) ReadSetBegin() (wire.Type, int32, error) {
	ttype, err := br.readBuffByte()
	if err != nil {
		return wire.Type(0), 0, err
	}

	length, err := br.readBuffI32()
	if err != nil {
		return wire.Type(0), 0, err
	}

	return wire.Type(ttype), length, nil
}

func (br *Reader) ReadSetEnd() error {
	return nil
}

func (br *Reader) ReadMapBegin() (wire.Type, wire.Type, int32, error) {
	ktype, err := br.readBuffByte()
	if err != nil {
		return wire.Type(0), wire.Type(0), 0, err
	}

	vtype, err := br.readBuffByte()
	if err != nil {
		return wire.Type(0), wire.Type(0), 0, err
	}

	length, err := br.readBuffI32()
	if err != nil {
		return wire.Type(0), wire.Type(0), 0, err
	}

	return wire.Type(ktype), wire.Type(vtype), length, nil
}

func (br *Reader) ReadMapEnd() error {
	return nil
}