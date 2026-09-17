package tools

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// The fake RPC server below answers with the same XML documents the FreeCAD
// addon produces, so the tools are exercised through the real XML-RPC client.

func rpcResponse(value string) string {
	return `<?xml version="1.0"?><methodResponse><params><param>` + value + `</param></params></methodResponse>`
}

func faultResponse(code int, message string) string {
	return `<?xml version="1.0"?><methodResponse><fault><value><struct>` +
		`<member><name>faultCode</name><value><int>` + strconv.Itoa(code) + `</int></value></member>` +
		`<member><name>faultString</name><value><string>` + message + `</string></value></member>` +
		`</struct></value></fault></methodResponse>`
}

func stringValue(text string) string {
	escaped := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;").Replace(text)
	return "<value><string>" + escaped + "</string></value>"
}

func boolValue(value bool) string {
	if value {
		return "<value><boolean>1</boolean></value>"
	}
	return "<value><boolean>0</boolean></value>"
}

func intValue(value int) string { return "<value><int>" + strconv.Itoa(value) + "</int></value>" }

// structValue builds a struct from alternating member names and raw <value> XML.
func structValue(members ...string) string {
	var b strings.Builder
	b.WriteString("<value><struct>")
	for i := 0; i+1 < len(members); i += 2 {
		b.WriteString("<member><name>")
		b.WriteString(members[i])
		b.WriteString("</name>")
		b.WriteString(members[i+1])
		b.WriteString("</member>")
	}
	b.WriteString("</struct></value>")
	return b.String()
}

// responses maps an RPC method to the raw <value> payload it answers with; ping
// always succeeds so tests only describe the method under test.
type responses map[string]string

func (r responses) handler(t *testing.T) func(string) string {
	return func(method string) string {
		if method == "ping" {
			return rpcResponse(boolValue(true))
		}
		value, ok := r[method]
		if !ok {
			t.Errorf("unexpected RPC method %q", method)
			return rpcResponse("<value><nil/></value>")
		}
		return rpcResponse(value)
	}
}

// requestLog records the XML-RPC calls a state made.
type requestLog struct {
	mu     sync.Mutex
	method []string
	body   []string
}

func (l *requestLog) record(method, body string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.method = append(l.method, method)
	l.body = append(l.body, body)
}

func (l *requestLog) lastBody() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	if len(l.body) == 0 {
		return ""
	}
	return l.body[len(l.body)-1]
}

// newState wires a State to a fake FreeCAD RPC server running on localhost.
func newState(t *testing.T, respond func(method string) string) (*State, *requestLog) {
	t.Helper()
	log := &requestLog{}
	rpcServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("reading request: %v", err)
		}
		method := methodName(string(body))
		log.record(method, string(body))
		w.Header().Set("Content-Type", "text/xml")
		io.WriteString(w, respond(method))
	}))
	t.Cleanup(rpcServer.Close)

	host, portText, err := net.SplitHostPort(strings.TrimPrefix(rpcServer.URL, "http://"))
	if err != nil {
		t.Fatalf("parsing the test server address: %v", err)
	}
	port, err := strconv.Atoi(portText)
	if err != nil {
		t.Fatalf("parsing the test server port: %v", err)
	}

	state := NewState()
	state.Host = host
	state.port = port
	t.Cleanup(state.Close)
	return state, log
}

// stateWith returns a state backed by a table of canned RPC responses.
func stateWith(t *testing.T, table responses) (*State, *requestLog) {
	t.Helper()
	return newState(t, table.handler(t))
}

func methodName(body string) string {
	const open, closeTag = "<methodName>", "</methodName>"
	start := strings.Index(body, open)
	if start < 0 {
		return ""
	}
	start += len(open)
	end := strings.Index(body[start:], closeTag)
	if end < 0 {
		return ""
	}
	return body[start : start+end]
}

func textOf(t *testing.T, result *mcp.CallToolResult) string {
	t.Helper()
	if len(result.Content) == 0 {
		t.Fatal("tool result has no content")
	}
	text, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatalf("first content is %T, want mcp.TextContent", result.Content[0])
	}
	return text.Text
}

func TestRegisterExposesThePythonToolSurface(t *testing.T) {
	srv := server.NewMCPServer("FreeCADMCP", "test")
	Register(srv, NewState())

	registered := srv.ListTools()
	names := make([]string, 0, len(registered))
	for name := range registered {
		names = append(names, name)
	}
	sort.Strings(names)

	want := []string{
		"create_document", "create_object", "delete_object", "edit_object",
		"execute_code", "execute_code_async", "execute_code_headless",
		"get_async_status", "get_object", "get_objects", "get_parts_list",
		"get_rpc_status", "get_view", "insert_part_from_library",
		"list_documents", "reload_document", "run_fem_analysis",
	}
	if !reflect.DeepEqual(names, want) {
		t.Errorf("tools = %#v, want %#v", names, want)
	}

	prompts := srv.ListPrompts()
	if _, ok := prompts["asset_creation_strategy"]; !ok || len(prompts) != 1 {
		t.Errorf("prompts = %#v, want only asset_creation_strategy", prompts)
	}

	// The default screenshot/view arguments must survive into the schema.
	createObject, ok := registered["create_object"]
	if !ok {
		t.Fatal("create_object is not registered")
	}
	properties := createObject.Tool.InputSchema.Properties
	if _, ok := properties["include_screenshot"]; !ok {
		t.Error("create_object is missing include_screenshot")
	}
	required := createObject.Tool.InputSchema.Required
	if !reflect.DeepEqual(required, []string{"doc_name", "obj_type", "obj_name"}) {
		t.Errorf("create_object required = %#v", required)
	}
	viewName, ok := properties["view_name"].(map[string]any)
	if !ok {
		t.Fatalf("view_name schema = %#v", properties["view_name"])
	}
	var enum []any
	switch typed := viewName["enum"].(type) {
	case []any:
		enum = typed
	case []string:
		for _, item := range typed {
			enum = append(enum, item)
		}
	}
	if len(enum) != 9 {
		t.Errorf("view_name enum = %#v, want the nine orientations", viewName["enum"])
	}
}

func TestRegistrationRejectsMissingRequiredArguments(t *testing.T) {
	srv := server.NewMCPServer("FreeCADMCP", "test")
	Register(srv, NewState())
	handler := srv.ListTools()["create_document"].Handler

	result, err := handler(context.Background(), mcp.CallToolRequest{
		Params: mcp.CallToolParams{Name: "create_document", Arguments: map[string]any{}},
	})
	if err != nil {
		t.Fatalf("handler returned a protocol error: %v", err)
	}
	if !result.IsError {
		t.Error("expected a tool error for a missing argument")
	}
	if got, want := textOf(t, result), `required argument "name" not found`; got != want {
		t.Errorf("text = %q, want %q", got, want)
	}
}

func TestConnectionFailureSurfacesAsToolError(t *testing.T) {
	// A server that answers every call with HTTP 500 makes the ping fail.
	broken := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(broken.Close)

	host, portText, err := net.SplitHostPort(strings.TrimPrefix(broken.URL, "http://"))
	if err != nil {
		t.Fatalf("parsing the test server address: %v", err)
	}
	port, _ := strconv.Atoi(portText)

	state := NewState()
	state.Host = host
	state.port = port

	result := state.CreateDocument(context.Background(), "MyDoc")
	if !result.IsError {
		t.Fatal("expected a tool error")
	}
	if got, want := textOf(t, result), "Failed to connect to FreeCAD. Make sure the FreeCAD addon is running."; got != want {
		t.Errorf("text = %q, want %q", got, want)
	}
}

func TestCreateDocumentMessages(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		state, _ := stateWith(t, responses{
			"create_document": structValue("success", boolValue(true), "document_name", stringValue("MyDoc")),
		})
		result := state.CreateDocument(context.Background(), "mydoc")
		if result.IsError {
			t.Fatalf("unexpected tool error: %s", textOf(t, result))
		}
		if got, want := textOf(t, result), "Document 'MyDoc' created successfully"; got != want {
			t.Errorf("text = %q, want %q", got, want)
		}
	})

	t.Run("reported failure", func(t *testing.T) {
		state, _ := stateWith(t, responses{
			"create_document": structValue("success", boolValue(false), "error", stringValue("document name in use")),
		})
		result := state.CreateDocument(context.Background(), "MyDoc")
		if got, want := textOf(t, result), "Failed to create document: document name in use"; got != want {
			t.Errorf("text = %q, want %q", got, want)
		}
	})
}

func TestCreateObjectSendsAnalysisAndProperties(t *testing.T) {
	table := responses{
		"create_object": structValue("success", boolValue(true), "object_name", stringValue("Box")),
	}
	state, log := stateWith(t, table)
	analysis := "Analysis"

	if result := state.CreateObject(
		context.Background(), "MyDoc", "Part::Box", "Box", nil, map[string]any{"Height": 30.0}, false, "Isometric",
	); result.IsError {
		t.Fatalf("unexpected tool error: %s", textOf(t, result))
	}
	body := log.lastBody()
	for _, want := range []string{
		"<methodName>create_object</methodName>",
		"<name>Name</name><value><string>Box</string></value>",
		"<name>Type</name><value><string>Part::Box</string></value>",
		"<name>Height</name><value><int>30</int></value>",
		// An omitted analysis reaches FreeCAD as Python None.
		"<name>Analysis</name><value><nil/></value>",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("create_object body missing %q:\n%s", want, body)
		}
	}

	if result := state.CreateObject(
		context.Background(), "MyDoc", "Part::Box", "Box", &analysis, nil, false, "Isometric",
	); result.IsError {
		t.Fatalf("unexpected tool error: %s", textOf(t, result))
	}
	body = log.lastBody()
	if !strings.Contains(body, "<name>Analysis</name><value><string>Analysis</string></value>") {
		t.Errorf("analysis was not forwarded:\n%s", body)
	}
	if !strings.Contains(body, "<name>Properties</name><value><struct></struct></value>") {
		t.Errorf("missing properties should be an empty struct:\n%s", body)
	}
}

func TestScreenshotFeedbackPolicy(t *testing.T) {
	table := responses{
		"create_object":         structValue("success", boolValue(true), "object_name", stringValue("Box")),
		"get_active_screenshot": stringValue("iVBORw0KGgo="),
	}

	t.Run("screenshot appended", func(t *testing.T) {
		state, _ := stateWith(t, table)
		result := state.CreateObject(context.Background(), "MyDoc", "Part::Box", "Box", nil, nil, true, "Front")
		if got, want := textOf(t, result), "Object 'Box' created successfully"; got != want {
			t.Errorf("text = %q, want %q", got, want)
		}
		if len(result.Content) != 2 {
			t.Fatalf("content = %#v, want text plus image", result.Content)
		}
		image, ok := result.Content[1].(mcp.ImageContent)
		if !ok {
			t.Fatalf("second content is %T, want mcp.ImageContent", result.Content[1])
		}
		if image.Data != "iVBORw0KGgo=" || image.MIMEType != "image/png" {
			t.Errorf("image = %#v", image)
		}
	})

	t.Run("text-only feedback", func(t *testing.T) {
		state, _ := stateWith(t, table)
		state.OnlyTextFeedback = true
		result := state.CreateObject(context.Background(), "MyDoc", "Part::Box", "Box", nil, nil, true, "Front")
		if len(result.Content) != 1 {
			t.Errorf("content = %#v, want text only", result.Content)
		}
	})

	t.Run("screenshot not requested", func(t *testing.T) {
		state, _ := stateWith(t, table)
		result := state.CreateObject(context.Background(), "MyDoc", "Part::Box", "Box", nil, nil, false, "Front")
		if len(result.Content) != 1 {
			t.Errorf("content = %#v, want text only", result.Content)
		}
	})
}

func TestListDocumentsFailureIsAToolError(t *testing.T) {
	state, _ := newState(t, func(method string) string {
		if method == "ping" {
			return rpcResponse(boolValue(true))
		}
		return faultResponse(1, "GUI_DISPATCH_FAILED: GUI thread busy")
	})

	result := state.ListDocuments(context.Background())
	if !result.IsError {
		t.Fatal("expected a tool error")
	}
	if got, want := textOf(t, result), "XML-RPC fault 1: GUI_DISPATCH_FAILED: GUI thread busy"; got != want {
		t.Errorf("text = %q, want %q", got, want)
	}
}

func TestListDocumentsReturnsJSON(t *testing.T) {
	state, _ := stateWith(t, responses{
		"list_documents": "<value><array><data>" + stringValue("MyDoc") + stringValue("其他") + "</data></array></value>",
	})
	result := state.ListDocuments(context.Background())
	if got, want := textOf(t, result), "[\n  \"MyDoc\",\n  \"其他\"\n]"; got != want {
		t.Errorf("text = %q, want %q", got, want)
	}
}

func TestGetViewReturnsAnImageOrAnExplanation(t *testing.T) {
	t.Run("screenshot", func(t *testing.T) {
		state, _ := stateWith(t, responses{"get_active_screenshot": stringValue("iVBORw0KGgo=")})
		result := state.GetView(context.Background(), "Isometric", nil, nil, nil)
		if len(result.Content) != 1 {
			t.Fatalf("content = %#v", result.Content)
		}
		image, ok := result.Content[0].(mcp.ImageContent)
		if !ok || image.Data != "iVBORw0KGgo=" {
			t.Errorf("content = %#v, want an image", result.Content[0])
		}
	})

	t.Run("unsupported view", func(t *testing.T) {
		state, _ := stateWith(t, responses{"get_active_screenshot": "<value><nil/></value>"})
		result := state.GetView(context.Background(), "Isometric", nil, nil, nil)
		want := "Cannot get screenshot in the current view type (such as TechDraw or Spreadsheet)"
		if got := textOf(t, result); got != want {
			t.Errorf("text = %q, want %q", got, want)
		}
	})
}

func TestGetAsyncStatusMessages(t *testing.T) {
	t.Run("single job", func(t *testing.T) {
		state, _ := stateWith(t, responses{
			"get_async_status": structValue(
				"success", boolValue(true),
				"job", structValue(
					"id", stringValue("job-1"),
					"state", stringValue("failed"),
					"error", stringValue("NameError"),
					"traceback", stringValue("Traceback (most recent call last):"),
				),
			),
		})
		result := state.GetAsyncStatus(context.Background(), "job-1")
		want := "Async job job-1: failed\nError: NameError\nTraceback (most recent call last):"
		if got := textOf(t, result); got != want {
			t.Errorf("text = %q, want %q", got, want)
		}
	})

	t.Run("job list", func(t *testing.T) {
		state, _ := stateWith(t, responses{
			"get_async_status": structValue(
				"success", boolValue(true),
				"jobs", "<value><array><data>"+structValue("id", stringValue("job-1"), "state", stringValue("running"))+"</data></array></value>",
			),
		})
		result := state.GetAsyncStatus(context.Background(), "")
		if got := textOf(t, result); !strings.Contains(got, `"id": "job-1"`) {
			t.Errorf("text = %q, want the job list as JSON", got)
		}
	})

	t.Run("reported failure", func(t *testing.T) {
		state, _ := stateWith(t, responses{
			"get_async_status": structValue("success", boolValue(false), "error", stringValue("job-1 not found")),
		})
		result := state.GetAsyncStatus(context.Background(), "job-1")
		if got, want := textOf(t, result), "Failed to get async status: job-1 not found"; got != want {
			t.Errorf("text = %q, want %q", got, want)
		}
	})
}

func TestExecuteCodeAsyncMessages(t *testing.T) {
	t.Run("with a job id", func(t *testing.T) {
		state, _ := stateWith(t, responses{
			"execute_code_async": structValue("success", boolValue(true), "job_id", stringValue("job-7")),
		})
		result := state.ExecuteCodeAsync(context.Background(), "pass")
		text := textOf(t, result)
		if !strings.Contains(text, "job_id: job-7") || !strings.Contains(text, `Poll get_async_status(job_id="job-7")`) {
			t.Errorf("text = %q", text)
		}
	})

	t.Run("without a job id", func(t *testing.T) {
		state, _ := stateWith(t, responses{"execute_code_async": structValue("success", boolValue(true))})
		result := state.ExecuteCodeAsync(context.Background(), "pass")
		if got := textOf(t, result); !strings.Contains(got, "use get_object to poll") {
			t.Errorf("text = %q", got)
		}
	})

	t.Run("reported failure without a message", func(t *testing.T) {
		state, _ := stateWith(t, responses{"execute_code_async": structValue("success", boolValue(false))})
		result := state.ExecuteCodeAsync(context.Background(), "pass")
		if got, want := textOf(t, result), "Failed to start async execution: unknown"; got != want {
			t.Errorf("text = %q, want %q", got, want)
		}
	})
}

func TestRunFEMAnalysisSummaries(t *testing.T) {
	summaryOf := func(t *testing.T, result *mcp.CallToolResult) string {
		t.Helper()
		var payload map[string]any
		if err := json.Unmarshal([]byte(textOf(t, result)), &payload); err != nil {
			t.Fatalf("result is not JSON: %v", err)
		}
		summary, _ := payload["summary"].(string)
		return summary
	}

	t.Run("solved", func(t *testing.T) {
		state, _ := stateWith(t, responses{
			"run_fem_analysis": structValue(
				"success", boolValue(true),
				"max_von_mises_MPa", "<value><double>12.34567</double></value>",
				"max_displacement_mm", "<value><double>0.5</double></value>",
				"node_count", intValue(1234),
			),
		})
		result := state.RunFEMAnalysis(context.Background(), "MyDoc", "Analysis", 600, false, "Isometric")
		want := "FEM analysis 'Analysis' solved. max von Mises = 12.35 MPa, max displacement = 0.5 mm (1234 nodes)."
		if got := summaryOf(t, result); got != want {
			t.Errorf("summary = %q, want %q", got, want)
		}
	})

	t.Run("missing solver values", func(t *testing.T) {
		state, _ := stateWith(t, responses{"run_fem_analysis": structValue("success", boolValue(true))})
		result := state.RunFEMAnalysis(context.Background(), "MyDoc", "Analysis", 600, false, "Isometric")
		want := "FEM analysis 'Analysis' solved. max von Mises = unavailable (MPa), " +
			"max displacement = unavailable (mm) (None nodes)."
		if got := summaryOf(t, result); got != want {
			t.Errorf("summary = %q, want %q", got, want)
		}
	})

	t.Run("failed", func(t *testing.T) {
		state, _ := stateWith(t, responses{
			"run_fem_analysis": structValue("success", boolValue(false), "error", stringValue("CalculiX not found")),
		})
		result := state.RunFEMAnalysis(context.Background(), "MyDoc", "Analysis", 600, false, "Isometric")
		want := "FEM analysis 'Analysis' failed: CalculiX not found"
		if got := summaryOf(t, result); got != want {
			t.Errorf("summary = %q, want %q", got, want)
		}
	})
}

func TestGetPartsListEmptyMessage(t *testing.T) {
	state, _ := stateWith(t, responses{
		"get_parts_list": "<value><array><data></data></array></value>",
	})
	result := state.GetPartsList(context.Background())
	want := "No parts found in the parts library. You must add parts_library addon."
	if got := textOf(t, result); got != want {
		t.Errorf("text = %q, want %q", got, want)
	}
}

func TestExecuteCodeHeadlessFormatsFailures(t *testing.T) {
	state, _ := stateWith(t, responses{})
	result := state.ExecuteCodeHeadless(context.Background(), "pass", -1)
	want := "Headless FreeCAD script FAILED: timeout must be a positive finite number"
	if got := textOf(t, result); got != want {
		t.Errorf("text = %q, want %q", got, want)
	}
}

func TestGetRPCStatusRendersPythonCompatibleJSON(t *testing.T) {
	state, _ := stateWith(t, responses{
		"get_rpc_status": structValue(
			"gui_dispatch", structValue("state", stringValue("healthy")),
			// HTML characters stay unescaped and UTF-8 is preserved, like
			// json.dumps(..., ensure_ascii=False).
			"note", stringValue("ok <done> 中文"),
		),
	})
	result := state.GetRPCStatus(context.Background())
	want := "{\n" +
		"  \"gui_dispatch\": {\n    \"state\": \"healthy\"\n  },\n" +
		"  \"note\": \"ok <done> 中文\"\n" +
		"}"
	if got := textOf(t, result); got != want {
		t.Errorf("text = %q, want %q", got, want)
	}
}

func TestReloadDocumentMessages(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		state, _ := stateWith(t, responses{
			"reload_document": structValue("success", boolValue(true), "document_name", stringValue("MyDoc")),
		})
		result := state.ReloadDocument(context.Background(), "MyDoc")
		if got, want := textOf(t, result), "Document 'MyDoc' reloaded from disk."; got != want {
			t.Errorf("text = %q, want %q", got, want)
		}
	})

	t.Run("reported failure", func(t *testing.T) {
		state, _ := stateWith(t, responses{
			"reload_document": structValue("success", boolValue(false), "error", stringValue("document is not saved")),
		})
		result := state.ReloadDocument(context.Background(), "MyDoc")
		if got, want := textOf(t, result), "Failed to reload document: document is not saved"; got != want {
			t.Errorf("text = %q, want %q", got, want)
		}
	})
}

func TestExecuteCodeMessages(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		state, _ := stateWith(t, responses{
			"execute_code": structValue("success", boolValue(true), "message", stringValue("Code executed")),
		})
		result := state.ExecuteCode(context.Background(), "pass", false, "Isometric")
		if got, want := textOf(t, result), "Code executed successfully: Code executed"; got != want {
			t.Errorf("text = %q, want %q", got, want)
		}
	})

	t.Run("python error", func(t *testing.T) {
		state, _ := stateWith(t, responses{
			"execute_code": structValue("success", boolValue(false), "error", stringValue("NameError: name 'foo' is not defined")),
		})
		result := state.ExecuteCode(context.Background(), "foo", false, "Isometric")
		want := "Failed to execute code: NameError: name 'foo' is not defined"
		if got := textOf(t, result); got != want {
			t.Errorf("text = %q, want %q", got, want)
		}
	})
}
