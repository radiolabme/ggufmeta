package main

import (
	"encoding/binary"
	"fmt"
	"io"
	"math"
)

type scanner struct {
	r     io.Reader
	order binary.ByteOrder
	pos   uint64
}

func newScanner(r io.Reader) *scanner { return &scanner{r: r} }

// readExact reads exactly n bytes and updates position - single source of truth
func (s *scanner) readExact(n int) ([]byte, error) {
	buf := make([]byte, n)
	_, err := io.ReadFull(s.r, buf)
	if err == nil {
		s.pos += uint64(n)
	}
	return buf, err
}

func (s *scanner) b(n int) ([]byte, error) {
	return s.readExact(n)
}

// All reads use single explicit path - no hidden buffering or position tracking
func (s *scanner) U8() (uint8, error) {
	b, e := s.readExact(1)
	if e != nil {
		return 0, e
	}
	return b[0], nil
}
func (s *scanner) I8() (int8, error) {
	b, e := s.readExact(1)
	if e != nil {
		return 0, e
	}
	return int8(b[0]), nil
}
func (s *scanner) U16() (uint16, error) {
	b, e := s.readExact(2)
	if e != nil {
		return 0, e
	}
	return s.order.Uint16(b), nil
}
func (s *scanner) I16() (int16, error) {
	b, e := s.readExact(2)
	if e != nil {
		return 0, e
	}
	return int16(s.order.Uint16(b)), nil
}
func (s *scanner) U32() (uint32, error) {
	b, e := s.readExact(4)
	if e != nil {
		return 0, e
	}
	return s.order.Uint32(b), nil
}
func (s *scanner) I32() (int32, error) {
	b, e := s.readExact(4)
	if e != nil {
		return 0, e
	}
	return int32(s.order.Uint32(b)), nil
}
func (s *scanner) U64() (uint64, error) {
	b, e := s.readExact(8)
	if e != nil {
		return 0, e
	}
	return s.order.Uint64(b), nil
}
func (s *scanner) I64() (int64, error) {
	b, e := s.readExact(8)
	if e != nil {
		return 0, e
	}
	return int64(s.order.Uint64(b)), nil
}
func (s *scanner) F32() (float32, error) { u, e := s.U32(); return math.Float32frombits(u), e }
func (s *scanner) F64() (float64, error) { u, e := s.U64(); return math.Float64frombits(u), e }

func (s *scanner) GGUFString(max uint64) (string, error) {
	n, err := s.U64()
	if err != nil {
		return "", err
	}
	if n > max {
		return "", fmt.Errorf("string too large: %d > %d", n, max)
	}
	buf, err := s.b(int(n))
	if err != nil {
		return "", err
	}
	return string(buf), nil
}

func (s *scanner) Align(n uint64) error {
	if n == 0 {
		return nil
	}
	if rem := s.pos % n; rem != 0 {
		need := int(n - rem)
		_, err := s.b(need)
		return err
	}
	return nil
}
// Align8 is a convenience method for 8-byte alignment.
// Used by the experimental --align-before-value flag.
func (s *scanner) Align8() error { return s.Align(8) }

// This scanner implementation follows Dijkstra's advice: "make the program so simple
// that there are obviously no deficiencies." The single readExact method eliminates
// position tracking bugs that plagued earlier versions with dual tracking systems.
