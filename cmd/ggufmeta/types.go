// Package main defines types and constants for GGUF v3 file format parsing.
// This file contains the core data structures and type definitions that match
// the official GGUF v3 specification for reading metadata from GGUF files.
package main

// GGUF v3 Reference - https://github.com/ggml-org/ggml/blob/master/docs/gguf.md

// magicGGUF is the 4-byte magic number that identifies GGUF files.
// All valid GGUF files must start with these exact bytes.
const magicGGUF = "GGUF"

// headerEvent represents the first output record containing GGUF file header information.
// This is emitted as the first line of NDJSON output to provide file structure overview.
type headerEvent struct {
	Kind string `json:"kind"` // Always "header" to identify this record type
	GGUF struct {
		Version     uint32 `json:"version"`     // GGUF format version (should be 3)
		TensorCount uint64 `json:"tensorCount"` // Number of tensors in the file
		KVCount     uint64 `json:"kvCount"`     // Number of key-value metadata pairs
	} `json:"gguf"`
}

// kvEvent represents a single key-value pair from the GGUF metadata section.
// Each KV pair is emitted as one line of NDJSON output after the header.
type kvEvent struct {
	Key   string      `json:"key"`   // The metadata key (e.g., "general.name", "tokenizer.ggml.tokens")
	Type  string      `json:"type"`  // Human-readable type description (e.g., "string", "array[int32]")
	Value interface{} `json:"value"` // The actual value or placeholder for large arrays
}

// GGUF type constants based on the official GGUF v3 specification.
// These numeric values are encoded in the file and must match exactly.
// The order and values here are critical for correct parsing.
const (
	tUint8   uint32 = 0  // 8-bit unsigned integer
	tInt8    uint32 = 1  // 8-bit signed integer
	tUint16  uint32 = 2  // 16-bit unsigned integer
	tInt16   uint32 = 3  // 16-bit signed integer
	tUint32  uint32 = 4  // 32-bit unsigned integer
	tInt32   uint32 = 5  // 32-bit signed integer
	tFloat32 uint32 = 6  // 32-bit IEEE 754 floating point
	tBool    uint32 = 7  // Boolean (stored as uint8: 0=false, non-zero=true)
	tString  uint32 = 8  // UTF-8 string with uint64 length prefix
	tArray   uint32 = 9  // Array with element type and uint64 count
	tUint64  uint32 = 10 // 64-bit unsigned integer
	tInt64   uint32 = 11 // 64-bit signed integer
	tFloat64 uint32 = 12 // 64-bit IEEE 754 floating point
)

// typeNames provides human-readable names for GGUF type constants.
// The array index corresponds to the type constant value.
// Used for generating user-friendly type descriptions in output.
var typeNames = []string{
	"uint8",   // 0 - tUint8
	"int8",    // 1 - tInt8
	"uint16",  // 2 - tUint16
	"int16",   // 3 - tInt16
	"uint32",  // 4 - tUint32
	"int32",   // 5 - tInt32
	"float32", // 6 - tFloat32
	"bool",    // 7 - tBool
	"string",  // 8 - tString
	"array",   // 9 - tArray
	"uint64",  // 10 - tUint64
	"int64",   // 11 - tInt64
	"float64", // 12 - tFloat64
}

// policy controls parsing behavior and output formatting decisions.
// This implements the two-pass strategy: show structure by default, expand selectively.
type policy struct {
	maxArray       uint64            // Arrays larger than this show placeholders instead of full content
	maxString      uint64            // Maximum string length to prevent memory exhaustion
	debug          bool              // Enable detailed debug output to stderr
	expandArrays   map[string]bool   // Exact array key names that should be expanded fully
	expandPrefixes []string          // Key prefixes that should have their arrays expanded (from "prefix.*")
}
