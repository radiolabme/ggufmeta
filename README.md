
⸻


# ggufmeta (pure Go)

A tiny, dependency-free CLI to extract **GGUF** model metadata into **NDJSON**.

- **Pure Go** (stdlib only) — no codegen, no Java, no Kaitai.
- **Stream-first**: reads header + metadata key/values; does **not** touch tensor payloads.
- Emits **NDJSON**: first line is a header event; following lines are one metadata entry per line.

> GGUF spec: https://github.com/ggml-org/ggml/blob/master/docs/gguf.md

## Install

Requires Go ≥ 1.20.

```bash
make
sudo make install

By default this installs ggufmeta to /usr/local/bin.

Usage

ggufmeta model.gguf

Example NDJSON:

{"kind":"header","gguf":{"version":3,"tensorCount":291,"kvCount":47}}
{"key":"general.architecture","type":"string","value":"llama"}
{"key":"general.name","type":"string","value":"MyModel"}
{"key":"tokenizer.ggml.add_bos_token","type":"bool","value":true}

Filter by key prefix:

ggufmeta --keys tokenizer. model.gguf

Shape NDJSON with jq

Fold to a single JSON object:

ggufmeta model.gguf \
| jq -s '.[0].gguf as $h
         | {gguf: ($h + {kv: (.[1:] | map({key, type, value}))})}'

Compact dictionary (keys → values):

ggufmeta model.gguf \
| jq -s '{gguf: (.[0].gguf + {kv: (.[1:] | map({key, value}) | from_entries)})}'

Get all keys:

ggufmeta model.gguf | jq -r 'select(.key) | .key'

Get one key’s value:

ggufmeta model.gguf | jq -r 'select(.key=="general.architecture") | .value' | head -n1

Notes
	•	Endianness: GGUF files are primarily little-endian; parser uses a safe heuristic on version to detect rare big-endian headers.
	•	Safety: strings/arrays are size-capped to avoid pathological inputs.

License

MIT — see LICENSE.

This utility parses the GGUF format as described by the ggml-org/ggml project (MIT-licensed). We reference the spec and do not include or modify their source code.
