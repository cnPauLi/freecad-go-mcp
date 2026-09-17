package freecad

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// rpcResponse wraps a raw <value> payload in a methodResponse document.
func rpcResponse(value string) string {
	return `<?xml version="1.0"?><methodResponse><params><param>` + value + `</param></params></methodResponse>`
}

func stringValue(text string) string { return "<value><string>" + text + "</string></value>" }

func boolValue(value bool) string {
	if value {
		return "<value><boolean>1</boolean></value>"
	}
	return "<value><boolean>0</boolean></value>"
}

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

// newTestClient points a client at a test server while keeping the production
// transport, so connection-pool behaviour stays representative.
func newTestClient(server *httptest.Server) *Client {
	client := NewClient("localhost", DefaultRPCPort, 0)
	client.uri = server.URL
	return client
}

func serveXMLRPC(t *testing.T, handler func(w http.ResponseWriter, body string)) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("reading request: %v", err)
		}
		w.Header().Set("Content-Type", "text/xml")
		handler(w, string(body))
	}))
}

func TestLongCallBudgetsMatchThePythonClient(t *testing.T) {
	defaultClient := NewClient("localhost", DefaultRPCPort, 0)
	cases := []struct {
		name string
		got  time.Duration
		want time.Duration
	}{
		{"execute_code covers a queue wait plus a full run", defaultClient.executeCodeBudget(), 210 * time.Second},
		{"run_fem_analysis(600) covers a queue wait plus a full solve", defaultClient.femBudget(600), 1230 * time.Second},
		{"a short fem timeout never shrinks the base budget", defaultClient.femBudget(60), 150 * time.Second},
	}
	for _, testCase := range cases {
		if testCase.got != testCase.want {
			t.Errorf("%s: budget = %s, want %s", testCase.name, testCase.got, testCase.want)
		}
	}

	// An explicitly larger socket budget is never reduced.
	large := NewClient("localhost", DefaultRPCPort, 2000*time.Second)
	if got := large.executeCodeBudget(); got != 2000*time.Second {
		t.Errorf("execute_code budget = %s, want 2000s", got)
	}
	if got := large.femBudget(600); got != 2000*time.Second {
		t.Errorf("run_fem_analysis budget = %s, want 2000s", got)
	}

	// A budget between the base and the long-call budgets wins.
	mid := NewClient("localhost", DefaultRPCPort, 500*time.Second)
	if got := mid.executeCodeBudget(); got != 500*time.Second {
		t.Errorf("execute_code budget = %s, want 500s", got)
	}
}

func TestCallPostsXMLRPCRequestAndDecodesStruct(t *testing.T) {
	var (
		path        string
		contentType string
		agent       string
		body        string
	)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		contentType = r.Header.Get("Content-Type")
		agent = r.Header.Get("User-Agent")
		received, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("reading request: %v", err)
		}
		body = string(received)
		w.Header().Set("Content-Type", "text/xml")
		io.WriteString(w, rpcResponse(structValue(
			"Name", stringValue("MyDoc"),
			"Created", boolValue(true),
			"VertexCount", "<value><int>8</int></value>",
		)))
	}))
	defer server.Close()

	result, err := newTestClient(server).CreateDocument(context.Background(), "MyDoc")
	if err != nil {
		t.Fatalf("CreateDocument: %v", err)
	}
	if path != "/RPC2" {
		t.Errorf("path = %q, want /RPC2", path)
	}
	if contentType != "text/xml" {
		t.Errorf("Content-Type = %q, want text/xml", contentType)
	}
	if agent != userAgent {
		t.Errorf("User-Agent = %q, want %q", agent, userAgent)
	}
	if !strings.Contains(body, "<methodName>create_document</methodName>") {
		t.Errorf("request body missing the method name:\n%s", body)
	}
	if !strings.Contains(body, "<string>MyDoc</string>") {
		t.Errorf("request body missing the argument:\n%s", body)
	}
	if GetString(result, "Name") != "MyDoc" || !GetBool(result, "Created") {
		t.Errorf("result = %#v", result)
	}
	if count, ok := GetNumber(result, "VertexCount"); !ok || count != 8 {
		t.Errorf("VertexCount = %v (ok=%v), want 8", count, ok)
	}
}

func TestCallSurfacesFaultsAndHTTPErrors(t *testing.T) {
	t.Run("fault", func(t *testing.T) {
		server := serveXMLRPC(t, func(w http.ResponseWriter, _ string) {
			io.WriteString(w, `<?xml version="1.0"?><methodResponse><fault><value><struct>`+
				`<member><name>faultCode</name><value><int>1</int></value></member>`+
				`<member><name>faultString</name><value><string>GUI_DISPATCH_FAILED: GUI thread busy</string></value></member>`+
				`</struct></value></fault></methodResponse>`)
		})
		defer server.Close()

		_, err := newTestClient(server).ExecuteCode(context.Background(), "pass")
		if err == nil || !strings.Contains(err.Error(), "XML-RPC fault 1: GUI_DISPATCH_FAILED: GUI thread busy") {
			t.Fatalf("err = %v, want the XML-RPC fault", err)
		}
	})

	t.Run("http error", func(t *testing.T) {
		server := serveXMLRPC(t, func(w http.ResponseWriter, _ string) {
			w.WriteHeader(http.StatusInternalServerError)
		})
		defer server.Close()

		_, err := newTestClient(server).CreateDocument(context.Background(), "MyDoc")
		if err == nil || !strings.Contains(err.Error(), "HTTP 500") {
			t.Fatalf("err = %v, want an HTTP 500 error", err)
		}
	})

	t.Run("unreachable server", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
		client := newTestClient(server)
		server.Close()

		if _, err := client.Ping(context.Background()); err == nil {
			t.Fatal("expected an error for a closed server")
		}
	})
}

func TestGetActiveScreenshotHandlesNilAndText(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		server := serveXMLRPC(t, func(w http.ResponseWriter, _ string) {
			io.WriteString(w, rpcResponse("<value><nil/></value>"))
		})
		defer server.Close()

		screenshot, err := newTestClient(server).GetActiveScreenshot(context.Background(), "Isometric", nil, nil, nil)
		if err != nil {
			t.Fatalf("GetActiveScreenshot: %v", err)
		}
		if screenshot != "" {
			t.Errorf("screenshot = %q, want an empty string", screenshot)
		}
	})

	t.Run("base64 text", func(t *testing.T) {
		server := serveXMLRPC(t, func(w http.ResponseWriter, _ string) {
			io.WriteString(w, rpcResponse(stringValue("iVBORw0KGgo=")))
		})
		defer server.Close()

		screenshot, err := newTestClient(server).GetActiveScreenshot(context.Background(), "Isometric", nil, nil, nil)
		if err != nil {
			t.Fatalf("GetActiveScreenshot: %v", err)
		}
		if screenshot != "iVBORw0KGgo=" {
			t.Errorf("screenshot = %q, want the encoded payload", screenshot)
		}
	})
}

func TestListDocumentsAndPartsListDecodeArrays(t *testing.T) {
	server := serveXMLRPC(t, func(w http.ResponseWriter, body string) {
		if strings.Contains(body, "get_parts_list") {
			io.WriteString(w, rpcResponse("<value><array><data>"+stringValue("Part/Box.FCStd")+stringValue("Part/Cone.FCStd")+"</data></array></value>"))
			return
		}
		io.WriteString(w, rpcResponse("<value><array><data>"+stringValue("MyDoc")+"</data></array></value>"))
	})
	defer server.Close()

	client := newTestClient(server)
	documents, err := client.ListDocuments(context.Background())
	if err != nil {
		t.Fatalf("ListDocuments: %v", err)
	}
	if len(documents) != 1 || documents[0] != "MyDoc" {
		t.Errorf("documents = %#v", documents)
	}
	parts, err := client.GetPartsList(context.Background())
	if err != nil {
		t.Fatalf("GetPartsList: %v", err)
	}
	if len(parts) != 2 || parts[1] != "Part/Cone.FCStd" {
		t.Errorf("parts = %#v", parts)
	}
}

func TestGetObjectsHandlesEmptyResult(t *testing.T) {
	server := serveXMLRPC(t, func(w http.ResponseWriter, _ string) {
		io.WriteString(w, rpcResponse("<value><array><data></data></array></value>"))
	})
	defer server.Close()

	objects, err := newTestClient(server).GetObjects(context.Background(), "MyDoc")
	if err != nil {
		t.Fatalf("GetObjects: %v", err)
	}
	if len(objects) != 0 {
		t.Errorf("objects = %#v, want none", objects)
	}
}

func TestBlockingExecuteCodeDoesNotStallOtherCalls(t *testing.T) {
	release := make(chan struct{})
	started := make(chan struct{}, 2)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("reading request: %v", err)
		}
		w.Header().Set("Content-Type", "text/xml")
		if strings.Contains(string(body), "<methodName>execute_code</methodName>") {
			// Hold the GUI-thread call until both callers are inside the handler.
			started <- struct{}{}
			<-release
			io.WriteString(w, rpcResponse(structValue("success", boolValue(true))))
			return
		}
		io.WriteString(w, rpcResponse(structValue("gui_dispatch", structValue("state", stringValue("healthy")))))
	}))
	defer server.Close()

	client := newTestClient(server)
	results := make(chan error, 2)
	for range 2 {
		go func() {
			_, err := client.ExecuteCode(context.Background(), "import time\ntime.sleep(0.7)")
			results <- err
		}()
	}
	for range 2 {
		select {
		case <-started:
		case <-time.After(5 * time.Second):
			t.Fatal("execute_code never reached the server")
		}
	}

	// A status query must be answered while both GUI-thread calls are blocked.
	statusCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	status, err := client.GetRPCStatus(statusCtx)
	if err != nil {
		t.Fatalf("get_rpc_status while execute_code is blocked: %v", err)
	}
	dispatch, ok := status["gui_dispatch"].(map[string]any)
	if !ok || GetString(dispatch, "state") != "healthy" {
		t.Errorf("status = %#v", status)
	}

	close(release)
	for range 2 {
		if err := <-results; err != nil {
			t.Errorf("execute_code: %v", err)
		}
	}
}
