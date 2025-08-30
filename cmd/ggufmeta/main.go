package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func envUint64(name string, def uint64) uint64 {
	if v := strings.TrimSpace(os.Getenv(name)); v != "" {
		if n, err := strconv.ParseUint(v, 10, 64); err == nil {
			return n
		}
	}
	return def
}
func envBool(name string, def bool) bool {
	if v := strings.TrimSpace(os.Getenv(name)); v != "" {
		switch strings.ToLower(v) {
		case "1", "true", "yes", "on":
			return true
		case "0", "false", "no", "off":
			return false
		}
	}
	return def
}

func safeCapFromCount(n uint64) int {
	const maxInt = int(^uint(0) >> 1)
	if n > uint64(maxInt) {
		return maxInt
	}
	return int(n)
}

func main() {
	log.SetFlags(0)

	var (
		keys         string
		maxArray     uint64
		maxString    uint64
		debug        bool
		tensors      bool
		tokens       bool
		expandArrays string
	)

	flag.StringVar(&keys, "keys", "", "show only KV pairs with keys matching this prefix (e.g., 'tokenizer.' for tokenizer.*, 'general.' for model info)")
	flag.Uint64Var(&maxArray, "max-array", envUint64("GGUF_META_MAX_ARRAY", 32), "threshold for large arrays - show placeholder instead of full content")
	flag.Uint64Var(&maxString, "max-string", envUint64("GGUF_META_MAX_STRING", 131072), "maximum string length (bytes)")
	flag.BoolVar(&debug, "debug", envBool("GGUF_META_DEBUG", false), "print debug info to stderr")
	flag.BoolVar(&tensors, "tensors", false, "include tensor-related KV pairs (*.weight, *.bias, etc.)")
	flag.BoolVar(&tokens, "tokens", false, "include tokenizer KV pairs (tokenizer.*)")
	flag.StringVar(&expandArrays, "expand-arrays", "", "comma-separated list of array keys to expand (e.g., 'general.special_tokens,tokenizer.ggml.added_tokens')")

	// NEW: let us flip the critical alignment rule at runtime
	flag.BoolVar(&alignBeforeValue, "align-before-value", false, "align to 8 before reading each value payload")

	flag.Parse()

	if flag.NArg() != 1 {
		fmt.Fprintf(os.Stderr, "usage: %s [options] file.gguf\n", filepath.Base(os.Args[0]))
		fmt.Fprintf(os.Stderr, "\nExtract GGUF metadata as NDJSON. By default, shows all keys with array placeholders.\n")
		fmt.Fprintf(os.Stderr, "\nOptions:\n")
		fmt.Fprintf(os.Stderr, "  --keys PREFIX        show only keys with this prefix (e.g., 'tokenizer.', 'general.')\n")
		fmt.Fprintf(os.Stderr, "  --tokens             (legacy flag, no effect - arrays show as placeholders by default)\n")
		fmt.Fprintf(os.Stderr, "  --tensors            (legacy flag, no effect - arrays show as placeholders by default)\n")
		fmt.Fprintf(os.Stderr, "  --max-array N        threshold for large arrays - show placeholder (default: 32)\n")
		fmt.Fprintf(os.Stderr, "  --max-string BYTES   maximum string length in bytes (default: 131072)\n")
		fmt.Fprintf(os.Stderr, "  --expand-arrays LIST comma-separated array keys to expand fully (overrides size limits)\n")
		fmt.Fprintf(os.Stderr, "  --debug              print debug info to stderr\n")
		fmt.Fprintf(os.Stderr, "  --align-before-value experimental alignment toggle\n")
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  %s model.gguf                              # show all metadata with array placeholders\n", filepath.Base(os.Args[0]))
		fmt.Fprintf(os.Stderr, "  %s --expand-arrays tokenizer.ggml.tokens   # expand specific arrays fully\n", filepath.Base(os.Args[0]))
		fmt.Fprintf(os.Stderr, "  %s --keys general. model.gguf              # show only general.* keys\n", filepath.Base(os.Args[0]))
		os.Exit(2)
	}

	path := flag.Arg(0)
	f, err := os.Open(path)
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		if err := f.Close(); err != nil {
			log.Printf("failed to close file: %v", err)
		}
	}()

	var fsize uint64
	if st, err := f.Stat(); err == nil && st.Mode().IsRegular() {
		fsize = uint64(st.Size())
	}

	// Parse expand-arrays parameter
	expandMap := make(map[string]bool)
	var expandPrefixes []string
	if expandArrays != "" {
		for _, pattern := range strings.Split(expandArrays, ",") {
			pattern = strings.TrimSpace(pattern)
			if strings.HasSuffix(pattern, "*") {
				// Pattern like "tokenizer.*" - treat as prefix
				expandPrefixes = append(expandPrefixes, strings.TrimSuffix(pattern, "*"))
			} else {
				// Exact key match
				expandMap[pattern] = true
			}
		}
	}

	pol := policy{
		maxArray:       maxArray,
		maxString:      maxString,
		debug:          debug,
		expandArrays:   expandMap,
		expandPrefixes: expandPrefixes,
	}

	p, hdr, err := newParser(f, fsize, pol)
	if err != nil {
		log.Fatal(err)
	}

	enc := json.NewEncoder(os.Stdout)
	_ = enc.Encode(hdr)

	// Define key filtering logic - now only filters based on --keys parameter
	matchKey := func(k string) bool {
		// If --keys is specified, use exact prefix matching
		if keys != "" {
			return strings.HasPrefix(k, strings.TrimSpace(keys))
		}
		// Default: show all keys
		return true
	}

	for {
		kv, ok, err := p.nextKV()
		if err != nil {
			log.Fatal(err)
		}
		if !ok {
			break
		}
		if kv.Key == "" { // omitted
			continue
		}
		if !matchKey(kv.Key) {
			continue
		}

		// For arrays, always show placeholder info by default
		// The --tokens and --tensors flags control whether to expand arrays, not whether to show them

		_ = enc.Encode(kv)
	}
}

// Command-line entry point for the GGUF metadata extraction tool.
// Parses GGUF v3 files and outputs metadata as NDJSON with configurable
// array expansion and key filtering capabilities.
