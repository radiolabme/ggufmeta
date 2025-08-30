
# ggufmeta (A pure Go GGUF metadata reader)

A tiny, dependency-free CLI to extract **GGUF** model metadata into **NDJSON**.

- **Pure Go**: Uses stdlib only with no further dependencies.
- **Stream-first**: reads header + metadata key/values; does **not** touch tensor payloads.
- Emits **NDJSON**: first line is a header event; following lines are one metadata entry per line.

> GGUF spec: https://github.com/ggml-org/ggml/blob/master/docs/gguf.md

## Install

Requires Go ≥ 1.20.

```bash
make
sudo make install
```

By default this installs ggufmeta to /usr/local/bin.

## Usage

usage: ggufmeta [options] file.gguf

Extract GGUF metadata as NDJSON. By default, shows all keys with array placeholders.

Options:
  --keys PREFIX        show only keys with this prefix (e.g., 'tokenizer.', 'general.')
  --tokens             (legacy flag, no effect - arrays show as placeholders by default)
  --tensors            (legacy flag, no effect - arrays show as placeholders by default)
  --max-array N        threshold for large arrays - show placeholder (default: 32)
  --max-string BYTES   maximum string length in bytes (default: 131072)
  --expand-arrays LIST comma-separated array keys to expand fully (overrides size limits)
  --debug              print debug info to stderr
  --align-before-value experimental alignment toggle

Examples:
  ggufmeta model.gguf                              # show all metadata with array placeholders
  ggufmeta --expand-arrays tokenizer.ggml.tokens   # expand specific arrays fully
  ggufmeta --keys general. model.gguf              # show only general.* keys

Example NDJSON:

bin/ggufmeta ../data/models/tinyllama-1.1b-chat-v1.0.Q4_K_M.gguf 
{"kind":"header","gguf":{"version":3,"tensorCount":201,"kvCount":23}}
{"key":"general.architecture","type":"string","value":"llama"}
{"key":"general.name","type":"string","value":"tinyllama_tinyllama-1.1b-chat-v1.0"}
{"key":"llama.context_length","type":"uint32","value":2048}
{"key":"llama.embedding_length","type":"uint32","value":2048}
{"key":"llama.block_count","type":"uint32","value":22}
{"key":"llama.feed_forward_length","type":"uint32","value":5632}
{"key":"llama.rope.dimension_count","type":"uint32","value":64}
{"key":"llama.attention.head_count","type":"uint32","value":32}
{"key":"llama.attention.head_count_kv","type":"uint32","value":4}
{"key":"llama.attention.layer_norm_rms_epsilon","type":"float32","value":0.00001}
{"key":"llama.rope.freq_base","type":"float32","value":10000}
{"key":"general.file_type","type":"uint32","value":15}
{"key":"tokenizer.ggml.model","type":"string","value":"llama"}
{"key":"tokenizer.ggml.tokens","type":"array[string]","value":{"_placeholder":"array","count":32000,"element_type":"string"}}
{"key":"tokenizer.ggml.scores","type":"array[float32]","value":{"_placeholder":"array","count":32000,"element_type":"float32"}}
{"key":"tokenizer.ggml.token_type","type":"array[int32]","value":{"_placeholder":"array","count":32000,"element_type":"int32"}}
{"key":"tokenizer.ggml.merges","type":"array[string]","value":{"_placeholder":"array","count":61249,"element_type":"string"}}
{"key":"tokenizer.ggml.bos_token_id","type":"uint32","value":1}
{"key":"tokenizer.ggml.eos_token_id","type":"uint32","value":2}
{"key":"tokenizer.ggml.unknown_token_id","type":"uint32","value":0}
{"key":"tokenizer.ggml.padding_token_id","type":"uint32","value":2}
{"key":"tokenizer.chat_template","type":"string","value":"{% for message in messages %}\n{% if message['role'] == 'user' %}\n{{ '\u003c|user|\u003e\n' + message['content'] + eos_token }}\n{% elif message['role'] == 'system' %}\n{{ '\u003c|system|\u003e\n' + message['content'] + eos_token }}\n{% elif message['role'] == 'assistant' %}\n{{ '\u003c|assistant|\u003e\n'  + message['content'] + eos_token }}\n{% endif %}\n{% if loop.last and add_generation_prompt %}\n{{ '\u003c|assistant|\u003e' }}\n{% endif %}\n{% endfor %}"}
{"key":"general.quantization_version","type":"uint32","value":2}

## Shape NDJSON with jq

### Fold to a single JSON object:

ggufmeta model.gguf \
| jq -s '.[0].gguf as $h
         | {gguf: ($h + {kv: (.[1:] | map({key, type, value}))})}'

### Compact dictionary (keys → values):

ggufmeta model.gguf \
| jq -s '{gguf: (.[0].gguf + {kv: (.[1:] | map({key, value}) | from_entries)})}'

### Get all keys:

ggufmeta model.gguf | jq -r 'select(.key) | .key'

### Get one key’s value:

ggufmeta model.gguf | jq -r 'select(.key=="general.architecture") | .value' | head -n1

## Notes
	•	Endianness: GGUF files are primarily little-endian; parser uses a safe heuristic on version to detect rare big-endian headers.
	•	Safety: strings/arrays are size-capped to avoid pathological inputs.
