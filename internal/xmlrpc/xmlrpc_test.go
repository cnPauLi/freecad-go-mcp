package xmlrpc

import (
	"strings"
	"testing"
)

func TestMarshalRequestEncodesNestedValues(t *testing.T) {
	body, err := MarshalRequest("create_object", []any{
		"MyDoc",
		map[string]any{
			"Name":       "Cylinder",
			"Analysis":   nil,
			"Properties": map[string]any{"Height": 30, "Radius": 10.5, "Visible": true},
			"References": []any{map[string]any{"face": "Face1"}},
		},
	})
	if err != nil {
		t.Fatalf("MarshalRequest: %v", err)
	}
	xml := string(body)
	for _, want := range []string{
		"<methodName>create_object</methodName>",
		`<struct>`,
		"<name>Name</name><value><string>Cylinder</string></value>",
		// An omitted analysis reaches FreeCAD as Python None.
		"<name>Analysis</name><value><nil/></value>",
		// JSON integers are sent as XML-RPC ints, decimals as doubles.
		"<name>Height</name><value><int>30</int></value>",
		"<name>Radius</name><value><double>10.5</double></value>",
		"<name>Visible</name><value><boolean>1</boolean></value>",
		"<value><array><data><value><struct>",
	} {
		if !strings.Contains(xml, want) {
			t.Errorf("request body missing %q\n%s", want, xml)
		}
	}
}

func TestMarshalRequestEscapesStrings(t *testing.T) {
	body, err := MarshalRequest("execute_code", []any{"print(\"a\" < 'b' & 'c')"})
	if err != nil {
		t.Fatalf("MarshalRequest: %v", err)
	}
	if !strings.Contains(string(body), "print(&#34;a&#34; &lt; &#39;b&#39; &amp; &#39;c&#39;)") {
		t.Errorf("string was not escaped for XML: %s", body)
	}
}

func TestUnmarshalResponseDecodesObject(t *testing.T) {
	response := `<?xml version="1.0"?>
<methodResponse><params><param><value><struct>
<member><name>Name</name><value><string>Box</string></value></member>
<member><name>Volume</name><value><double>1000.0</double></value></member>
<member><name>VertexCount</name><value><int>8</int></value></member>
<member><name>Shape</name><value><nil/></value></member>
<member><name>Properties</name><value><struct>
<member><name>ShapeColor</name><value><array><data>
<value><double>0.5</double></value><value><double>0.5</double></value>
</data></array></value></member>
</struct></value></member>
</struct></value></param></params></methodResponse>`

	value, err := UnmarshalResponse([]byte(response))
	if err != nil {
		t.Fatalf("UnmarshalResponse: %v", err)
	}
	object, ok := value.(map[string]any)
	if !ok {
		t.Fatalf("expected a struct, got %T", value)
	}
	if got := object["Name"]; got != "Box" {
		t.Errorf("Name = %#v, want Box", got)
	}
	if got := object["Volume"]; got != 1000.0 {
		t.Errorf("Volume = %#v, want 1000.0", got)
	}
	if got := object["VertexCount"]; got != int64(8) {
		t.Errorf("VertexCount = %#v, want int64(8)", got)
	}
	if got, present := object["Shape"]; !present || got != nil {
		t.Errorf("Shape = %#v, want nil", got)
	}
	properties, ok := object["Properties"].(map[string]any)
	if !ok {
		t.Fatalf("Properties = %T, want a struct", object["Properties"])
	}
	colors, ok := properties["ShapeColor"].([]any)
	if !ok || len(colors) != 2 {
		t.Fatalf("ShapeColor = %#v, want two doubles", properties["ShapeColor"])
	}
}

func TestUnmarshalResponseDecodesScalarsAndEmptyValue(t *testing.T) {
	cases := map[string]any{
		`<value><string>text</string></value>`:        "text",
		`<value><boolean>0</boolean></value>`:         false,
		`<value><boolean>1</boolean></value>`:         true,
		`<value><int>-3</int></value>`:                int64(-3),
		`<value><double>2.5</double></value>`:         2.5,
		`<value></value>`:                             "",
		`<value><array><data></data></array></value>`: []any{},
	}
	for fragment, want := range cases {
		body := `<?xml version="1.0"?><methodResponse><params><param>` + fragment +
			`</param></params></methodResponse>`
		value, err := UnmarshalResponse([]byte(body))
		if err != nil {
			t.Errorf("UnmarshalResponse(%s): %v", fragment, err)
			continue
		}
		if !equalValue(value, want) {
			t.Errorf("UnmarshalResponse(%s) = %#v, want %#v", fragment, value, want)
		}
	}
}

func TestUnmarshalResponseSurfacesFault(t *testing.T) {
	response := `<?xml version="1.0"?><methodResponse><fault><value><struct>
<member><name>faultCode</name><value><int>1</int></value></member>
<member><name>faultString</name><value><string>GUI_DISPATCH_FAILED: GUI thread busy</string></value></member>
</struct></value></fault></methodResponse>`

	_, err := UnmarshalResponse([]byte(response))
	fault, ok := err.(*Fault)
	if !ok {
		t.Fatalf("expected a *Fault, got %v", err)
	}
	if fault.Code != 1 || fault.String != "GUI_DISPATCH_FAILED: GUI thread busy" {
		t.Errorf("fault = %#v", fault)
	}
}

func TestUnmarshalResponseRejectsMalformedBody(t *testing.T) {
	if _, err := UnmarshalResponse([]byte("<html>not xml-rpc</html>")); err == nil {
		t.Error("expected an error for a non XML-RPC body")
	}
}

func TestMarshalRequestEncodesPointersAndTypedMaps(t *testing.T) {
	analysis := "Analysis"
	var missing *int
	body, err := MarshalRequest("create_object", []any{
		&analysis,
		missing,
		map[string]string{"Unit": "mm"},
	})
	if err != nil {
		t.Fatalf("MarshalRequest: %v", err)
	}
	xml := string(body)
	for _, want := range []string{
		"<value><string>Analysis</string></value>",
		"<value><nil/></value>",
		"<name>Unit</name><value><string>mm</string></value>",
	} {
		if !strings.Contains(xml, want) {
			t.Errorf("request body missing %q\n%s", want, xml)
		}
	}
}

func equalValue(got, want any) bool {
	switch typed := want.(type) {
	case []any:
		items, ok := got.([]any)
		return ok && len(items) == len(typed)
	default:
		return got == want
	}
}
