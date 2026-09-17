// Package xmlrpc implements the subset of the XML-RPC protocol used to talk to
// the FreeCAD addon's RPC server (see addon/FreeCADMCP/rpc_server/rpc_server.py).
//
// The Python server runs with allow_none=True, so nil values are encoded with
// the <nil/> extension. Structs are decoded into map[string]any and arrays into
// []any; integers become int64 and doubles become float64.
package xmlrpc

import (
	"bytes"
	"encoding/base64"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"math"
	"reflect"
	"sort"
	"strconv"
	"strings"
)

// Fault is an XML-RPC fault response.
type Fault struct {
	Code   int
	String string
}

func (f *Fault) Error() string {
	return fmt.Sprintf("XML-RPC fault %d: %s", f.Code, f.String)
}

// MarshalRequest encodes a method call as a methodCall document.
func MarshalRequest(method string, params []any) ([]byte, error) {
	var b bytes.Buffer
	b.WriteString("<?xml version=\"1.0\"?>\n")
	b.WriteString("<methodCall><methodName>")
	writeEscaped(&b, method)
	b.WriteString("</methodName><params>")
	for _, param := range params {
		b.WriteString("<param>")
		if err := writeValue(&b, param); err != nil {
			return nil, err
		}
		b.WriteString("</param>")
	}
	b.WriteString("</params></methodCall>")
	return b.Bytes(), nil
}

// UnmarshalResponse decodes a methodResponse document. A fault is returned as a
// *Fault error, matching the exception the Python client would raise.
func UnmarshalResponse(data []byte) (any, error) {
	var response methodResponse
	decoder := xml.NewDecoder(bytes.NewReader(data))
	if err := decoder.Decode(&response); err != nil {
		return nil, fmt.Errorf("malformed XML-RPC response: %w", err)
	}
	if response.Fault != nil {
		return nil, response.Fault.fault()
	}
	if response.Params == nil || len(response.Params.Param) == 0 {
		return nil, errors.New("malformed XML-RPC response: no parameters")
	}
	return response.Params.Param[0].Value.V, nil
}

type methodResponse struct {
	Params *struct {
		Param []parameter `xml:"param"`
	} `xml:"params"`
	Fault *valueElement `xml:"fault"`
}

type parameter struct {
	Value valueElement `xml:"value"`
}

// valueElement decodes an XML-RPC <value> element into a Go value.
type valueElement struct {
	V any
}

func (v *valueElement) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	var decoded any
	haveValue := false
	for {
		token, err := d.Token()
		if err == io.EOF {
			return io.ErrUnexpectedEOF
		}
		if err != nil {
			return err
		}
		switch t := token.(type) {
		case xml.StartElement:
			if haveValue {
				if err := d.Skip(); err != nil {
					return err
				}
				continue
			}
			value, err := decodeTyped(d, t)
			if err != nil {
				return err
			}
			decoded = value
			haveValue = true
		case xml.CharData:
			// Whitespace-only content between elements is formatting.
			text := string(t)
			if !haveValue && strings.TrimSpace(text) != "" {
				decoded = text
				haveValue = true
			}
		case xml.EndElement:
			// <value></value> with no content decodes as an empty string.
			if !haveValue {
				decoded = ""
			}
			v.V = decoded
			return nil
		}
	}
}

func decodeTyped(d *xml.Decoder, start xml.StartElement) (any, error) {
	switch start.Name.Local {
	case "nil":
		if err := d.Skip(); err != nil {
			return nil, err
		}
		return nil, nil
	case "value":
		// A <value> nested inside another element (e.g. <fault><value>) is
		// unwrapped transparently.
		var inner valueElement
		if err := d.DecodeElement(&inner, &start); err != nil {
			return nil, err
		}
		return inner.V, nil
	case "string", "name", "dateTime.iso8601":
		return decodeText(d, start)
	case "int", "i4", "i8", "biginteger":
		text, err := decodeText(d, start)
		if err != nil {
			return nil, err
		}
		number, err := strconv.ParseInt(strings.TrimSpace(text), 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid XML-RPC integer %q: %w", text, err)
		}
		return number, nil
	case "double":
		text, err := decodeText(d, start)
		if err != nil {
			return nil, err
		}
		number, err := strconv.ParseFloat(strings.TrimSpace(text), 64)
		if err != nil {
			return nil, fmt.Errorf("invalid XML-RPC double %q: %w", text, err)
		}
		return number, nil
	case "boolean":
		text, err := decodeText(d, start)
		if err != nil {
			return nil, err
		}
		return strings.TrimSpace(text) == "1" || strings.EqualFold(strings.TrimSpace(text), "true"), nil
	case "base64":
		text, err := decodeText(d, start)
		if err != nil {
			return nil, err
		}
		raw, err := base64.StdEncoding.DecodeString(strings.Join(strings.Fields(text), ""))
		if err != nil {
			return nil, fmt.Errorf("invalid XML-RPC base64 payload: %w", err)
		}
		return string(raw), nil
	case "array":
		var array struct {
			Data struct {
				Values []valueElement `xml:"value"`
			} `xml:"data"`
		}
		if err := d.DecodeElement(&array, &start); err != nil {
			return nil, err
		}
		items := make([]any, 0, len(array.Data.Values))
		for _, item := range array.Data.Values {
			items = append(items, item.V)
		}
		return items, nil
	case "struct":
		var structure struct {
			Members []struct {
				Name  string       `xml:"name"`
				Value valueElement `xml:"value"`
			} `xml:"member"`
		}
		if err := d.DecodeElement(&structure, &start); err != nil {
			return nil, err
		}
		members := make(map[string]any, len(structure.Members))
		for _, member := range structure.Members {
			members[member.Name] = member.Value.V
		}
		return members, nil
	default:
		// Unknown tag: fall back to its text content, like Python's decoder
		// treats unrecognised scalars.
		return decodeText(d, start)
	}
}

func decodeText(d *xml.Decoder, start xml.StartElement) (string, error) {
	var text string
	if err := d.DecodeElement(&text, &start); err != nil {
		return "", err
	}
	return text, nil
}

// fault converts a decoded fault struct into a *Fault error.
func (v valueElement) fault() error {
	fault := &Fault{}
	members, ok := v.V.(map[string]any)
	if !ok {
		return &Fault{String: fmt.Sprintf("%v", v.V)}
	}
	if code, ok := members["faultCode"]; ok {
		fault.Code = int(asInt64(code))
	}
	if message, ok := members["faultString"].(string); ok {
		fault.String = message
	}
	return fault
}

func writeValue(b *bytes.Buffer, v any) error {
	b.WriteString("<value>")
	if err := writeTyped(b, v); err != nil {
		return err
	}
	b.WriteString("</value>")
	return nil
}

func writeTyped(b *bytes.Buffer, v any) error {
	if v == nil {
		b.WriteString("<nil/>")
		return nil
	}
	switch typed := v.(type) {
	case bool:
		if typed {
			b.WriteString("<boolean>1</boolean>")
		} else {
			b.WriteString("<boolean>0</boolean>")
		}
	case string:
		b.WriteString("<string>")
		writeEscaped(b, typed)
		b.WriteString("</string>")
	case int:
		fmt.Fprintf(b, "<int>%d</int>", typed)
	case int8:
		fmt.Fprintf(b, "<int>%d</int>", typed)
	case int16:
		fmt.Fprintf(b, "<int>%d</int>", typed)
	case int32:
		fmt.Fprintf(b, "<int>%d</int>", typed)
	case int64:
		fmt.Fprintf(b, "<int>%d</int>", typed)
	case uint:
		fmt.Fprintf(b, "<int>%d</int>", typed)
	case uint8:
		fmt.Fprintf(b, "<int>%d</int>", typed)
	case uint16:
		fmt.Fprintf(b, "<int>%d</int>", typed)
	case uint32:
		fmt.Fprintf(b, "<int>%d</int>", typed)
	case uint64:
		fmt.Fprintf(b, "<int>%d</int>", typed)
	case float32:
		writeDouble(b, float64(typed))
	case float64:
		writeDouble(b, typed)
	case map[string]any:
		return writeStruct(b, typed)
	default:
		return writeReflect(b, v)
	}
	return nil
}

// writeDouble encodes a float. MCP arguments arrive as float64 because JSON has
// a single number type; an integral value is sent as <int> so FreeCAD sees the
// same Python type that a JSON integer literal produced for the Python client.
func writeDouble(b *bytes.Buffer, value float64) {
	if value == math.Trunc(value) && !math.IsInf(value, 0) && math.Abs(value) < 1<<53 {
		fmt.Fprintf(b, "<int>%d</int>", int64(value))
		return
	}
	b.WriteString("<double>")
	b.WriteString(strconv.FormatFloat(value, 'g', -1, 64))
	b.WriteString("</double>")
}

func writeStruct(b *bytes.Buffer, members map[string]any) error {
	b.WriteString("<struct>")
	for _, key := range sortedKeys(members) {
		b.WriteString("<member><name>")
		writeEscaped(b, key)
		b.WriteString("</name>")
		if err := writeValue(b, members[key]); err != nil {
			return err
		}
		b.WriteString("</member>")
	}
	b.WriteString("</struct>")
	return nil
}

// writeReflect encodes values that are not one of the concrete types handled by
// writeTyped: pointers and interfaces are dereferenced (nil becomes <nil/>) and
// named types are encoded by their underlying kind.
func writeReflect(b *bytes.Buffer, v any) error {
	value := reflect.ValueOf(v)
	for value.Kind() == reflect.Pointer || value.Kind() == reflect.Interface {
		if value.IsNil() {
			b.WriteString("<nil/>")
			return nil
		}
		value = value.Elem()
	}
	switch value.Kind() {
	case reflect.String:
		b.WriteString("<string>")
		writeEscaped(b, value.String())
		b.WriteString("</string>")
	case reflect.Bool:
		if value.Bool() {
			b.WriteString("<boolean>1</boolean>")
		} else {
			b.WriteString("<boolean>0</boolean>")
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		fmt.Fprintf(b, "<int>%d</int>", value.Int())
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		fmt.Fprintf(b, "<int>%d</int>", value.Uint())
	case reflect.Float32, reflect.Float64:
		writeDouble(b, value.Float())
	case reflect.Slice, reflect.Array:
		b.WriteString("<array><data>")
		for i := 0; i < value.Len(); i++ {
			if err := writeValue(b, value.Index(i).Interface()); err != nil {
				return err
			}
		}
		b.WriteString("</data></array>")
	case reflect.Map:
		if value.Type().Key().Kind() != reflect.String {
			return fmt.Errorf("unsupported XML-RPC map key type %s", value.Type().Key())
		}
		members := make(map[string]any, value.Len())
		iter := value.MapRange()
		for iter.Next() {
			members[iter.Key().String()] = iter.Value().Interface()
		}
		return writeStruct(b, members)
	default:
		return fmt.Errorf("unsupported XML-RPC value type %T", v)
	}
	return nil
}

func writeEscaped(b *bytes.Buffer, text string) {
	_ = xml.EscapeText(b, []byte(text))
}

// sortedKeys keeps struct member order deterministic across runs.
func sortedKeys(members map[string]any) []string {
	keys := make([]string, 0, len(members))
	for key := range members {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func asInt64(v any) int64 {
	switch typed := v.(type) {
	case int64:
		return typed
	case int:
		return int64(typed)
	case float64:
		return int64(typed)
	case string:
		number, err := strconv.ParseInt(typed, 10, 64)
		if err == nil {
			return number
		}
	}
	return 0
}
