// Package main implements GGUF value parsing with the two-pass strategy.
// This file handles reading and interpreting GGUF values (scalars, strings, arrays)
// with support for selective array expansion based on user preferences.
package main

import (
	"fmt"
	"os"
	"strings"
)

// alignBeforeValue is an experimental toggle for GGUF format alignment behavior.
// When true: align to 8-byte boundary before reading value payload after type tag
// When false: read value immediately after type tag (standard GGUF behavior)
// This helps debug GGUF files that may have non-standard alignment requirements.
var alignBeforeValue bool

// scalarDec defines the function signature for scalar value decoders.
// Each GGUF scalar type has a decoder that reads from the scanner.
type scalarDec = func(*scanner) (any, error)

// scalars maps GGUF type constants to their corresponding decoder functions.
// The array index corresponds to the type constant (tUint8=0, tInt8=1, etc.)
// nil entries indicate types that need special handling (string, array).
var scalars = []scalarDec{
	func(s *scanner) (any, error) { return s.U8() },                    // uint8   (0) - tUint8
	func(s *scanner) (any, error) { return s.I8() },                    // int8    (1) - tInt8
	func(s *scanner) (any, error) { return s.U16() },                   // uint16  (2) - tUint16
	func(s *scanner) (any, error) { return s.I16() },                   // int16   (3) - tInt16
	func(s *scanner) (any, error) { return s.U32() },                   // uint32  (4) - tUint32
	func(s *scanner) (any, error) { return s.I32() },                   // int32   (5) - tInt32
	func(s *scanner) (any, error) { return s.F32() },                   // float32 (6) - tFloat32
	func(s *scanner) (any, error) { u, e := s.U8(); return u != 0, e }, // bool    (7) - tBool (0=false, non-zero=true)
	nil, // string (8) - tString: special case handled in readScalar
	nil, // array (9) - tArray: special case handled in readValue
	func(s *scanner) (any, error) { return s.U64() }, // uint64  (10) - tUint64
	func(s *scanner) (any, error) { return s.I64() }, // int64   (11) - tInt64
	func(s *scanner) (any, error) { return s.F64() }, // float64 (12) - tFloat64
}

// typeLabel creates human-readable type descriptions for NDJSON output.
// Combines the base type name with optional shape information (e.g., "array[int32]").
func typeLabel(tag uint32, shape string) string {
	if int(tag) < len(typeNames) {
		// Known type - use the predefined name
		if shape == "" {
			return typeNames[tag]
		}
		// Add shape information (for arrays)
		return typeNames[tag] + shape
	}
	// Unknown type - show the raw tag value
	if shape == "" {
		return fmt.Sprintf("unknown(%d)", tag)
	}
	return fmt.Sprintf("unknown(%d)%s", tag, shape)
}

// readScalar reads a scalar value (non-array) from the GGUF file.
// Handles strings specially due to their length-prefixed format.
// Returns the value, type label, and any error.
func (p *parser) readScalar(tag uint32) (any, string, error) {
	if tag == tString {
		// Strings are special: uint64 length + UTF-8 bytes
		s, err := p.scn.GGUFString(p.pol.maxString)
		if err != nil {
			return nil, "", err
		}
		// No alignment after string - GGUF uses tight packing
		return s, "string", nil
	}
	// Validate the scalar type tag
	if int(tag) >= len(scalars) || scalars[tag] == nil {
		return nil, "", fmt.Errorf("bad scalar tag %d", tag)
	}
	// Use the appropriate decoder for this scalar type
	v, err := scalars[tag](p.scn)
	if err != nil {
		return nil, "", err
	}
	// No alignment after scalar - GGUF uses consecutive packing
	return v, typeLabel(tag, ""), nil
}

// readArray implements the two-pass strategy for array handling.
// By default, returns placeholders for arrays. Expands arrays only when explicitly requested.
// This prevents memory issues with large arrays while allowing selective detail access.
func (p *parser) readArray(key string) (any, string, bool, error) {
	// Read array header: element_type(u32) + count(u64)
	et, err := p.scn.U32()
	if err != nil {
		return nil, "", false, err
	}
	n, err := p.scn.U64()
	if err != nil {
		return nil, "", false, err
	}

	// Get human-readable name for the element type
	elemName := "unknown"
	if int(et) < len(typeNames) {
		elemName = typeNames[et]
	}

	// Debug output for array structure
	if p.pol.debug {
		fmt.Fprintf(os.Stderr, "[debug] key=%q array elemTag=%d(%s) len=%d pos=%d\n",
			key, et, elemName, n, p.scn.pos)
	}

	// Determine if this array should be expanded based on user preferences
	// Explicit expansion overrides size limits ("explicit should preempt implicit behavior")
	shouldExpand := p.pol.expandArrays[key] // Check exact key match first
	if !shouldExpand {
		// Check wildcard prefix matches (e.g., "tokenizer.*")
		for _, prefix := range p.pol.expandPrefixes {
			if strings.HasPrefix(key, prefix) {
				shouldExpand = true
				break
			}
		}
	}

	if shouldExpand {
		// User explicitly requested this array - expand it fully
		result, typeLabel, err := p.readExpandedArray(et, n, elemName)
		return result, typeLabel, false, err
	}

	// Default behavior: skip array contents efficiently and return placeholder
	err = p.bulkSkipArrayElements(et, n)
	if err != nil {
		return nil, "", false, err
	}

	// Create placeholder with structural information
	// This gives users the array metadata without the memory cost
	placeholder := map[string]any{
		"_placeholder": "array",    // Identifies this as a placeholder
		"count":        n,           // Number of elements
		"element_type": elemName,    // Type of each element
	}

	return placeholder, "array[" + elemName + "]", false, nil
}

// readExpandedArray reads and returns the full array contents when explicitly requested.
// This is used when users want to see actual array values instead of placeholders.
// Handles nested arrays by showing them as placeholders to prevent exponential expansion.
func (p *parser) readExpandedArray(elementType uint32, count uint64, elemName string) (any, string, error) {
	// Pre-allocate slice with safe capacity conversion
	results := make([]any, 0, safeCapFromCount(count))

	// Read each array element
	for i := uint64(0); i < count; i++ {
		if elementType == tArray {
			// Nested arrays: read structure but don't expand recursively
			// This prevents exponential memory usage with deeply nested arrays
			nestedET, err := p.scn.U32()
			if err != nil {
				return nil, "", err
			}
			nestedN, err := p.scn.U64()
			if err != nil {
				return nil, "", err
			}
			// Skip the nested array contents
			if err := p.bulkSkipArrayElements(nestedET, nestedN); err != nil {
				return nil, "", err
			}
			// Add placeholder for the nested array
			results = append(results, map[string]any{
				"_placeholder": "nested_array",
				"count":        nestedN,
				"element_type": typeNames[nestedET],
			})
		} else {
			// Scalar element - read the actual value
			v, _, err := p.readScalar(elementType)
			if err != nil {
				return nil, "", err
			}
			results = append(results, v)
		}
	}

	return results, "array[" + elemName + "]", nil
}

// bulkSkipArrayElements efficiently skips over array elements without storing values.
// This is the performance-critical path for large arrays that aren't being expanded.
// Uses iterative approach to avoid stack overflow on deeply nested arrays.
func (p *parser) bulkSkipArrayElements(elementType uint32, count uint64) error {
	for i := uint64(0); i < count; i++ {
		if elementType == tArray {
			// Nested array - read its header then skip its contents recursively
			nestedET, err := p.scn.U32()
			if err != nil {
				return err
			}
			nestedN, err := p.scn.U64()
			if err != nil {
				return err
			}
			// Recursively skip nested array elements
			if err := p.bulkSkipArrayElements(nestedET, nestedN); err != nil {
				return err
			}
		} else {
			// Scalar element - read it and discard (just for position advancement)
			_, _, err := p.readScalar(elementType)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// readValue is the main entry point for reading any GGUF value type.
// Handles the experimental alignment toggle and delegates to appropriate readers.
// Returns value, type label, omitted flag, and error.
func (p *parser) readValue(tag uint32, key string) (any, string, bool, error) {
	// EXPERIMENTAL ALIGNMENT TOGGLE:
	// Most GGUF files use tight packing (alignBeforeValue=false)
	// Some non-standard files may need 8-byte alignment before values (alignBeforeValue=true)
	if alignBeforeValue {
		if err := p.scn.Align8(); err != nil {
			return nil, "", false, err
		}
	}
	
	if tag == tArray {
		// Arrays need special handling due to two-pass strategy
		return p.readArray(key)
	}
	// All other types are scalars (including strings)
	v, typ, err := p.readScalar(tag)
	return v, typ, false, err
}

// This value parsing system implements the two-pass strategy:
// 1. Show all keys with array placeholders by default (fast, memory-efficient)
// 2. Expand specific arrays when explicitly requested (detailed, selective)
// The philosophy is "explicit should preempt implicit behavior" - user choices override defaults.
