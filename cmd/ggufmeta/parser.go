// cmd/ggufmeta/parser.go
package main

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
)

type parser struct {
	scn        *scanner
	fileSize   uint64
	endianHint string
	kvRemain   uint64
	version    uint32
	tc, kv     uint64
	pol        policy
}

func newParser(r io.Reader, size uint64, pol policy) (*parser, headerEvent, error) {
	scn := newScanner(r)

	// Read exactly 24 bytes of GGUF v3 header directly
	headerBytes, err := scn.readExact(24)
	if err != nil {
		return nil, headerEvent{}, fmt.Errorf("failed to read GGUF header: %w", err)
	}

	// Parse magic (bytes 0-3)
	if string(headerBytes[0:4]) != magicGGUF {
		return nil, headerEvent{}, fmt.Errorf("bad magic: got %q, expected %q", string(headerBytes[0:4]), magicGGUF)
	}

	// Parse version and detect endianness (bytes 4-7)
	versionLE := binary.LittleEndian.Uint32(headerBytes[4:8])
	versionBE := binary.BigEndian.Uint32(headerBytes[4:8])

	var version uint32
	var endianness string
	if versionLE == 3 {
		scn.order = binary.LittleEndian
		version = 3
		endianness = "LE"
	} else if versionBE == 3 {
		scn.order = binary.BigEndian
		version = 3
		endianness = "BE"
	} else {
		return nil, headerEvent{}, fmt.Errorf("unsupported GGUF version: LE=%d, BE=%d (expected 3)", versionLE, versionBE)
	}

	// Parse tensor count (bytes 8-15)
	tc := scn.order.Uint64(headerBytes[8:16])

	// Parse KV count (bytes 16-23)
	kv := scn.order.Uint64(headerBytes[16:24])

	if pol.debug {
		fmt.Fprintf(os.Stderr, "[debug] magic=%s version=%d endian=%s tensors=%d kvs=%d pos=%d\n",
			string(headerBytes[0:4]), version, endianness, tc, kv, scn.pos)
	}

	p := &parser{
		scn:        scn,
		fileSize:   size,
		endianHint: endianness,
		kvRemain:   kv,
		version:    version,
		tc:         tc,
		kv:         kv,
		pol:        pol,
	}

	var hdr headerEvent
	hdr.Kind = "header"
	hdr.GGUF.Version = version
	hdr.GGUF.TensorCount = tc
	hdr.GGUF.KVCount = kv
	return p, hdr, nil
}

func (p *parser) nextKV() (kvEvent, bool, error) {
	if p.kvRemain == 0 {
		return kvEvent{}, false, nil
	}

	// key (GGUF string) - KV pairs are packed consecutively
	key, err := p.scn.GGUFString(p.pol.maxString)
	if err != nil {
		return kvEvent{}, false, err
	}

	// tag (u32) immediately follows key (no alignment padding)
	tag, err := p.scn.U32()
	if err != nil {
		return kvEvent{}, false, fmt.Errorf("key %q: %w", key, err)
	}

	// read value (no pre-align)
	val, typ, omitted, err := p.readValue(tag, key)
	if err != nil {
		return kvEvent{}, false, fmt.Errorf("key %q: %w", key, err)
	}
	p.kvRemain--

	if omitted {
		return kvEvent{}, true, nil
	}
	// Return the complete key-value event for NDJSON output
	return kvEvent{Key: key, Type: typ, Value: val}, true, nil
}

// The parser coordinates between the low-level scanner (binary reading)
// and high-level value interpretation, implementing the GGUF v3 specification
// with robust error handling and endianness support.
