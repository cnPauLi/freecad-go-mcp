// Package freecad talks to the FreeCAD addon's XML-RPC server, which listens on
// port 9875 by default (addon/FreeCADMCP/rpc_server/rpc_server.py).
package freecad

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math"
	"net/http"
	"time"

	"github.com/cnPauLi/freecad-go-mcp/internal/xmlrpc"
)

const (
	// DefaultRPCPort is the port the FreeCAD addon binds.
	DefaultRPCPort = 9875

	// DefaultTimeout is the socket budget used by the Python client, expressed
	// as time.Duration.
	DefaultTimeout = 150 * time.Second

	// ExecuteCodeTimeout mirrors FreeCADRPC.EXECUTE_CODE_TIMEOUT in the addon:
	// the GUI thread runs the script for at most this long.
	ExecuteCodeTimeout = 90 * time.Second

	// RPCTimeoutMargin is added on top of doubled run budgets because a caller
	// may wait for the GUI queue before the run itself starts.
	RPCTimeoutMargin = 30 * time.Second

	userAgent = "freecad-go-mcp/0.1"
)

// Client is an XML-RPC client for a single FreeCAD RPC server.
//
// Every call runs on its own HTTP request. The transport opens an additional
// connection while a long GUI-thread call is in flight, so get_rpc_status and
// get_async_status keep answering even when execute_code is blocked -- the
// reason the Python client used a separate proxy for those calls.
type Client struct {
	uri     string
	http    *http.Client
	timeout time.Duration
}

// NewClient creates a client for host:port with the given base socket budget
// (DefaultTimeout when timeout <= 0).
func NewClient(host string, port int, timeout time.Duration) *Client {
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	transport := &http.Transport{
		Proxy:                 nil,
		MaxIdleConns:          16,
		MaxIdleConnsPerHost:   8,
		IdleConnTimeout:       90 * time.Second,
		ResponseHeaderTimeout: 0,
	}
	return &Client{
		uri:     fmt.Sprintf("http://%s:%d", host, port),
		http:    &http.Client{Transport: transport},
		timeout: timeout,
	}
}

// Disconnect releases idle HTTP connections held by the client.
func (c *Client) Disconnect() {
	c.http.CloseIdleConnections()
}

// call performs one XML-RPC request with the given socket budget.
func (c *Client) call(ctx context.Context, timeout time.Duration, method string, params ...any) (any, error) {
	body, err := xmlrpc.MarshalRequest(method, params)
	if err != nil {
		return nil, fmt.Errorf("encoding %s request: %w", method, err)
	}
	callCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	request, err := http.NewRequestWithContext(callCtx, http.MethodPost, c.uri+"/RPC2", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("building %s request: %w", method, err)
	}
	request.Header.Set("Content-Type", "text/xml")
	request.Header.Set("User-Agent", userAgent)

	response, err := c.http.Do(request)
	if err != nil {
		if callCtx.Err() != nil {
			return nil, fmt.Errorf("%s timed out after %s", method, timeout)
		}
		return nil, fmt.Errorf("%s failed: %w", method, err)
	}
	defer response.Body.Close()

	payload, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("reading %s response: %w", method, err)
	}
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s returned HTTP %d", method, response.StatusCode)
	}
	value, err := xmlrpc.UnmarshalResponse(payload)
	if err != nil {
		return nil, err
	}
	return value, nil
}

// callMap performs a call whose result must be an XML-RPC struct.
func (c *Client) callMap(ctx context.Context, timeout time.Duration, method string, params ...any) (map[string]any, error) {
	value, err := c.call(ctx, timeout, method, params...)
	if err != nil {
		return nil, err
	}
	result, err := toMap(value)
	if err != nil {
		return nil, fmt.Errorf("%s returned an unexpected result: %w", method, err)
	}
	return result, nil
}

// Ping reports whether the RPC server answers.
func (c *Client) Ping(ctx context.Context) (bool, error) {
	value, err := c.call(ctx, c.timeout, "ping")
	if err != nil {
		return false, err
	}
	return toBool(value), nil
}

// GetRPCStatus reports RPC and GUI-dispatch health without using the GUI thread.
func (c *Client) GetRPCStatus(ctx context.Context) (map[string]any, error) {
	return c.callMap(ctx, c.timeout, "get_rpc_status")
}

// GetAsyncStatus reports the state of background jobs started by ExecuteCodeAsync.
func (c *Client) GetAsyncStatus(ctx context.Context, jobID string) (map[string]any, error) {
	return c.callMap(ctx, c.timeout, "get_async_status", jobID)
}

// CreateDocument creates a new document and reports its actual (sanitised) name.
func (c *Client) CreateDocument(ctx context.Context, name string) (map[string]any, error) {
	return c.callMap(ctx, c.timeout, "create_document", name)
}

// CreateObject creates an object from the request payload built by the caller.
func (c *Client) CreateObject(ctx context.Context, docName string, objData map[string]any) (map[string]any, error) {
	return c.callMap(ctx, c.timeout, "create_object", docName, objData)
}

// EditObject applies properties to an existing object.
func (c *Client) EditObject(ctx context.Context, docName, objName string, objData map[string]any) (map[string]any, error) {
	return c.callMap(ctx, c.timeout, "edit_object", docName, objName, objData)
}

// DeleteObject removes an object from a document.
func (c *Client) DeleteObject(ctx context.Context, docName, objName string) (map[string]any, error) {
	return c.callMap(ctx, c.timeout, "delete_object", docName, objName)
}

// ReloadDocument closes and re-opens a saved document to pick up external edits.
func (c *Client) ReloadDocument(ctx context.Context, docName string) (map[string]any, error) {
	return c.callMap(ctx, c.timeout, "reload_document", docName)
}

// InsertPartFromLibrary inserts a part from the parts library addon.
func (c *Client) InsertPartFromLibrary(ctx context.Context, relativePath string) (map[string]any, error) {
	return c.callMap(ctx, c.timeout, "insert_part_from_library", relativePath)
}

// ExecuteCode runs Python code on the FreeCAD GUI thread.
func (c *Client) ExecuteCode(ctx context.Context, code string) (map[string]any, error) {
	return c.callMap(ctx, c.executeCodeBudget(), "execute_code", code)
}

// ExecuteCodeAsync starts code in a background thread and returns its job id.
func (c *Client) ExecuteCodeAsync(ctx context.Context, code string) (map[string]any, error) {
	return c.callMap(ctx, c.timeout, "execute_code_async", code)
}

// GetActiveScreenshot returns a base64-encoded PNG of the active view, or an
// empty string when the active view type does not support screenshots.
func (c *Client) GetActiveScreenshot(
	ctx context.Context, viewName string, width, height *int, focusObject *string,
) (string, error) {
	value, err := c.call(ctx, c.timeout, "get_active_screenshot", viewName, width, height, focusObject)
	if err != nil {
		return "", err
	}
	if value == nil {
		return "", nil
	}
	text, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("get_active_screenshot returned an unexpected result %T", value)
	}
	return text, nil
}

// GetObjects returns every object in a document.
func (c *Client) GetObjects(ctx context.Context, docName string) ([]any, error) {
	value, err := c.call(ctx, c.timeout, "get_objects", docName)
	if err != nil {
		return nil, err
	}
	return toSlice(value)
}

// GetObject returns one object from a document, or nil when it does not exist.
func (c *Client) GetObject(ctx context.Context, docName, objName string) (any, error) {
	return c.call(ctx, c.timeout, "get_object", docName, objName)
}

// GetPartsList returns the relative paths of the parts library.
func (c *Client) GetPartsList(ctx context.Context) ([]string, error) {
	value, err := c.call(ctx, c.timeout, "get_parts_list")
	if err != nil {
		return nil, err
	}
	return toStrings(value)
}

// ListDocuments returns the names of the open documents.
func (c *Client) ListDocuments(ctx context.Context) ([]string, error) {
	value, err := c.call(ctx, c.timeout, "list_documents")
	if err != nil {
		return nil, err
	}
	return toStrings(value)
}

// RunFEMAnalysis runs CalculiX on an existing analysis container.
func (c *Client) RunFEMAnalysis(ctx context.Context, docName, analysisName string, timeout int) (map[string]any, error) {
	return c.callMap(ctx, c.femBudget(timeout), "run_fem_analysis", docName, analysisName, timeout)
}

// executeCodeBudget allows for a full GUI-queue wait followed by a full run.
func (c *Client) executeCodeBudget() time.Duration {
	return max(c.timeout, 2*ExecuteCodeTimeout+RPCTimeoutMargin)
}

// femBudget allows for queueing and solving to consume `timeout` seconds each.
func (c *Client) femBudget(timeout int) time.Duration {
	return max(c.timeout, 2*time.Duration(timeout)*time.Second+RPCTimeoutMargin)
}

func toMap(value any) (map[string]any, error) {
	result, ok := value.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("expected a struct, got %T", value)
	}
	return result, nil
}

func toSlice(value any) ([]any, error) {
	if value == nil {
		return []any{}, nil
	}
	items, ok := value.([]any)
	if !ok {
		return nil, fmt.Errorf("expected an array, got %T", value)
	}
	return items, nil
}

func toStrings(value any) ([]string, error) {
	items, err := toSlice(value)
	if err != nil {
		return nil, err
	}
	texts := make([]string, 0, len(items))
	for _, item := range items {
		text, ok := item.(string)
		if !ok {
			return nil, fmt.Errorf("expected strings, got %T", item)
		}
		texts = append(texts, text)
	}
	return texts, nil
}

func toBool(value any) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	case int64:
		return typed != 0
	case float64:
		return typed != 0
	default:
		return value != nil
	}
}

// GetString reads a string member, returning "" when it is absent or of another
// type.
func GetString(members map[string]any, key string) string {
	text, _ := members[key].(string)
	return text
}

// GetBool reads a boolean member, returning false when it is absent.
func GetBool(members map[string]any, key string) bool {
	return toBool(members[key])
}

// GetNumber reads a numeric member; ok is false when it is absent or nil, which
// is how the addon reports a value it could not compute.
func GetNumber(members map[string]any, key string) (float64, bool) {
	switch typed := members[key].(type) {
	case int64:
		return float64(typed), true
	case int:
		return float64(typed), true
	case float64:
		if math.IsNaN(typed) || math.IsInf(typed, 0) {
			return 0, false
		}
		return typed, true
	default:
		return 0, false
	}
}
