package mcp

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// jsonObject is a JSON object that remembers key order. Go maps do not, and
// re-encoding ~/.claude.json from a map would reorder ~40 keys of unrelated
// session state on every sync — a huge diff in a file the tool rewrites
// constantly. Values stay as RawMessage so anything arc does not understand is
// carried through byte-for-byte.
type jsonObject struct {
	keys   []string
	values map[string]json.RawMessage
}

func newJSONObject() *jsonObject {
	return &jsonObject{values: map[string]json.RawMessage{}}
}

// decodeJSONObject parses a JSON object preserving key order. Empty input
// yields an empty object so a missing or blank provider file merges cleanly.
func decodeJSONObject(data []byte) (*jsonObject, error) {
	obj := newJSONObject()
	if len(bytes.TrimSpace(data)) == 0 {
		return obj, nil
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}
	if delim, ok := tok.(json.Delim); !ok || delim != '{' {
		return nil, fmt.Errorf("expected a JSON object, got %v", tok)
	}
	for dec.More() {
		keyTok, err := dec.Token()
		if err != nil {
			return nil, err
		}
		key, ok := keyTok.(string)
		if !ok {
			return nil, fmt.Errorf("expected an object key, got %v", keyTok)
		}
		var raw json.RawMessage
		if err := dec.Decode(&raw); err != nil {
			return nil, err
		}
		obj.set(key, raw)
	}
	if _, err := dec.Token(); err != nil {
		return nil, err
	}
	return obj, nil
}

func (o *jsonObject) get(key string) (json.RawMessage, bool) {
	v, ok := o.values[key]
	return v, ok
}

// set replaces an existing key in place, keeping its original position.
func (o *jsonObject) set(key string, value json.RawMessage) {
	if _, exists := o.values[key]; !exists {
		o.keys = append(o.keys, key)
	}
	o.values[key] = value
}

func (o *jsonObject) delete(key string) {
	if _, exists := o.values[key]; !exists {
		return
	}
	delete(o.values, key)
	for i, k := range o.keys {
		if k == key {
			o.keys = append(o.keys[:i], o.keys[i+1:]...)
			break
		}
	}
}

func (o *jsonObject) len() int { return len(o.keys) }

// encode renders the object as indented JSON in the original key order.
func (o *jsonObject) encode() ([]byte, error) {
	var compact bytes.Buffer
	compact.WriteByte('{')
	for i, k := range o.keys {
		if i > 0 {
			compact.WriteByte(',')
		}
		encKey, err := json.Marshal(k)
		if err != nil {
			return nil, err
		}
		compact.Write(encKey)
		compact.WriteByte(':')
		var value bytes.Buffer
		if err := json.Compact(&value, o.values[k]); err != nil {
			return nil, err
		}
		compact.Write(value.Bytes())
	}
	compact.WriteByte('}')

	var out bytes.Buffer
	if err := json.Indent(&out, compact.Bytes(), "", "  "); err != nil {
		return nil, err
	}
	out.WriteByte('\n')
	return out.Bytes(), nil
}
