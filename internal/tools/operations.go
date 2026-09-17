package tools

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"unicode"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/cnPauLi/freecad-go-mcp/internal/freecad"
)

// readyClient returns the FreeCAD connection or, when it cannot be reached, the
// error result the tool returns. The Python tools called get_freecad_connection()
// outside their guarded operation body, so a connection failure surfaced as a
// tool error rather than as "Failed to ..." text.
func (s *State) readyClient(ctx context.Context) (*freecad.Client, *mcp.CallToolResult) {
	client, err := s.Connection(ctx)
	if err != nil {
		return nil, mcp.NewToolResultError(err.Error())
	}
	return client, nil
}

// optionalScreenshot fetches a screenshot unless feedback is text-only or the
// caller skipped it. The boolean reports whether screenshots are skipped.
func (s *State) optionalScreenshot(
	ctx context.Context, client *freecad.Client, includeScreenshot bool, viewName string,
) (string, bool) {
	skip := s.OnlyTextFeedback || !includeScreenshot
	if skip {
		return "", true
	}
	return activeScreenshot(ctx, client, viewName), false
}

// activeScreenshot mirrors FreeCADConnection.get_active_screenshot: a failure is
// logged and reported as "no screenshot" instead of failing the whole tool call.
func activeScreenshot(ctx context.Context, client *freecad.Client, viewName string) string {
	data, err := client.GetActiveScreenshot(ctx, viewName, nil, nil, nil)
	if err != nil {
		Logger.Printf("Error getting screenshot: %v", err)
		return ""
	}
	return data
}

// CreateDocument creates a new document in FreeCAD.
func (s *State) CreateDocument(ctx context.Context, name string) *mcp.CallToolResult {
	client, failure := s.readyClient(ctx)
	if failure != nil {
		return failure
	}
	res, err := client.CreateDocument(ctx, name)
	if err != nil {
		Logger.Printf("Failed to create document: %v", err)
		return textResponse("Failed to create document: " + err.Error())
	}
	if freecad.GetBool(res, "success") {
		return textResponse(fmt.Sprintf("Document '%s' created successfully", freecad.GetString(res, "document_name")))
	}
	return textResponse("Failed to create document: " + freecad.GetString(res, "error"))
}

// CreateObject creates an object in a document.
func (s *State) CreateObject(
	ctx context.Context,
	docName, objType, objName string,
	analysisName *string,
	properties map[string]any,
	includeScreenshot bool,
	viewName string,
) *mcp.CallToolResult {
	client, failure := s.readyClient(ctx)
	if failure != nil {
		return failure
	}
	objData := map[string]any{
		"Name":       objName,
		"Type":       objType,
		"Properties": orEmptyMap(properties),
		"Analysis":   analysisName,
	}
	res, err := client.CreateObject(ctx, docName, objData)
	if err != nil {
		Logger.Printf("Failed to create object: %v", err)
		return textResponse("Failed to create object: " + err.Error())
	}
	if !freecad.GetBool(res, "success") {
		return textResponse("Failed to create object: " + freecad.GetString(res, "error"))
	}
	response := textResponse(fmt.Sprintf("Object '%s' created successfully", freecad.GetString(res, "object_name")))
	screenshot, skip := s.optionalScreenshot(ctx, client, includeScreenshot, viewName)
	return addScreenshotIfAvailable(response, screenshot, skip)
}

// EditObject applies properties to an existing object.
func (s *State) EditObject(
	ctx context.Context, docName, objName string, properties map[string]any,
	includeScreenshot bool, viewName string,
) *mcp.CallToolResult {
	client, failure := s.readyClient(ctx)
	if failure != nil {
		return failure
	}
	res, err := client.EditObject(ctx, docName, objName, map[string]any{"Properties": orEmptyMap(properties)})
	if err != nil {
		Logger.Printf("Failed to edit object: %v", err)
		return textResponse("Failed to edit object: " + err.Error())
	}
	if !freecad.GetBool(res, "success") {
		return textResponse("Failed to edit object: " + freecad.GetString(res, "error"))
	}
	response := textResponse(fmt.Sprintf("Object '%s' edited successfully", freecad.GetString(res, "object_name")))
	screenshot, skip := s.optionalScreenshot(ctx, client, includeScreenshot, viewName)
	return addScreenshotIfAvailable(response, screenshot, skip)
}

// DeleteObject removes an object from a document.
func (s *State) DeleteObject(
	ctx context.Context, docName, objName string, includeScreenshot bool, viewName string,
) *mcp.CallToolResult {
	client, failure := s.readyClient(ctx)
	if failure != nil {
		return failure
	}
	res, err := client.DeleteObject(ctx, docName, objName)
	if err != nil {
		Logger.Printf("Failed to delete object: %v", err)
		return textResponse("Failed to delete object: " + err.Error())
	}
	if !freecad.GetBool(res, "success") {
		return textResponse("Failed to delete object: " + freecad.GetString(res, "error"))
	}
	response := textResponse(fmt.Sprintf("Object '%s' deleted successfully", freecad.GetString(res, "object_name")))
	screenshot, skip := s.optionalScreenshot(ctx, client, includeScreenshot, viewName)
	return addScreenshotIfAvailable(response, screenshot, skip)
}

// ExecuteCode runs Python code on FreeCAD's GUI thread.
func (s *State) ExecuteCode(
	ctx context.Context, code string, includeScreenshot bool, viewName string,
) *mcp.CallToolResult {
	client, failure := s.readyClient(ctx)
	if failure != nil {
		return failure
	}
	res, err := client.ExecuteCode(ctx, code)
	if err != nil {
		Logger.Printf("Failed to execute code: %v", err)
		return textResponse("Failed to execute code: " + err.Error())
	}
	if !freecad.GetBool(res, "success") {
		return textResponse("Failed to execute code: " + freecad.GetString(res, "error"))
	}
	response := textResponse("Code executed successfully: " + freecad.GetString(res, "message"))
	// Only screenshot once the code completed: skipping on failure avoids a
	// second hanging call while a worker thread may still be running.
	screenshot, skip := s.optionalScreenshot(ctx, client, includeScreenshot, viewName)
	return addScreenshotIfAvailable(response, screenshot, skip)
}

// ExecuteCodeAsync starts code in a background thread and returns its job id.
func (s *State) ExecuteCodeAsync(ctx context.Context, code string) *mcp.CallToolResult {
	client, failure := s.readyClient(ctx)
	if failure != nil {
		return failure
	}
	res, err := client.ExecuteCodeAsync(ctx, code)
	if err != nil {
		Logger.Printf("Failed to start async code execution: %v", err)
		return textResponse("Failed to start async code execution: " + err.Error())
	}
	if !freecad.GetBool(res, "success") {
		message := freecad.GetString(res, "error")
		if message == "" {
			message = "unknown"
		}
		return textResponse("Failed to start async execution: " + message)
	}
	jobID := freecad.GetString(res, "job_id")
	if jobID == "" {
		return textResponse(
			"Code execution started in background.\n" +
				"This addon does not report job IDs; use get_object to poll " +
				"a document status object and inspect FreeCAD's Report View for errors.")
	}
	return textResponse(fmt.Sprintf(
		"Code execution started in background (job_id: %s).\n"+
			"Poll get_async_status(job_id=\"%s\") for state, error and traceback. "+
			"FreeCAD's Report View shows printed output when done.", jobID, jobID))
}

// ExecuteCodeHeadless runs a script in a separate freecadcmd process.
func (s *State) ExecuteCodeHeadless(ctx context.Context, code string, timeout float64) *mcp.CallToolResult {
	return textResponse(formatHeadlessResult(freecad.RunHeadless(ctx, code, timeout, s.FreeCADCmd)))
}

// GetAsyncStatus reports the state of background jobs.
func (s *State) GetAsyncStatus(ctx context.Context, jobID string) *mcp.CallToolResult {
	client, failure := s.readyClient(ctx)
	if failure != nil {
		return failure
	}
	res, err := client.GetAsyncStatus(ctx, jobID)
	if err != nil {
		Logger.Printf("Failed to get async status: %v", err)
		return textResponse("Failed to get async status: " + err.Error())
	}
	if !freecad.GetBool(res, "success") {
		message := freecad.GetString(res, "error")
		if message == "" {
			message = "unknown"
		}
		return textResponse("Failed to get async status: " + message)
	}
	job, ok := res["job"].(map[string]any)
	if !ok {
		jobs := res["jobs"]
		if jobs == nil {
			jobs = []any{}
		}
		return jsonResponse(jobs)
	}
	state := freecad.GetString(job, "state")
	if state == "" {
		state = "unknown"
	}
	text := fmt.Sprintf("Async job %s: %s", freecad.GetString(job, "id"), state)
	if jobError := freecad.GetString(job, "error"); jobError != "" {
		text += "\nError: " + jobError
	}
	if traceback := freecad.GetString(job, "traceback"); traceback != "" {
		text += "\n" + traceback
	}
	return textResponse(text)
}

// GetView returns a screenshot of the active view.
func (s *State) GetView(
	ctx context.Context, viewName string, width, height *int, focusObject *string,
) *mcp.CallToolResult {
	client, failure := s.readyClient(ctx)
	if failure != nil {
		return failure
	}
	data, err := client.GetActiveScreenshot(ctx, viewName, width, height, focusObject)
	if err != nil {
		Logger.Printf("Failed to get view: %v", err)
		return textResponse("Failed to get view: " + err.Error())
	}
	if data == "" {
		return textResponse("Cannot get screenshot in the current view type (such as TechDraw or Spreadsheet)")
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.ImageContent{Type: mcp.ContentTypeImage, Data: data, MIMEType: "image/png"},
		},
	}
}

// InsertPartFromLibrary inserts a part from the parts library addon.
func (s *State) InsertPartFromLibrary(
	ctx context.Context, relativePath string, includeScreenshot bool, viewName string,
) *mcp.CallToolResult {
	client, failure := s.readyClient(ctx)
	if failure != nil {
		return failure
	}
	res, err := client.InsertPartFromLibrary(ctx, relativePath)
	if err != nil {
		Logger.Printf("Failed to insert part from library: %v", err)
		return textResponse("Failed to insert part from library: " + err.Error())
	}
	if !freecad.GetBool(res, "success") {
		return textResponse("Failed to insert part from library: " + freecad.GetString(res, "error"))
	}
	response := textResponse("Part inserted from library: " + freecad.GetString(res, "message"))
	screenshot, skip := s.optionalScreenshot(ctx, client, includeScreenshot, viewName)
	return addScreenshotIfAvailable(response, screenshot, skip)
}

// GetObjects returns every object in a document.
func (s *State) GetObjects(
	ctx context.Context, docName string, includeScreenshot bool, viewName string,
) *mcp.CallToolResult {
	client, failure := s.readyClient(ctx)
	if failure != nil {
		return failure
	}
	objects, err := client.GetObjects(ctx, docName)
	if err != nil {
		Logger.Printf("Failed to get objects: %v", err)
		return textResponse("Failed to get objects: " + err.Error())
	}
	response := jsonResponse(objects)
	screenshot, skip := s.optionalScreenshot(ctx, client, includeScreenshot, viewName)
	return addScreenshotIfAvailable(response, screenshot, skip)
}

// GetObject returns one object from a document.
func (s *State) GetObject(
	ctx context.Context, docName, objName string, includeScreenshot bool, viewName string,
) *mcp.CallToolResult {
	client, failure := s.readyClient(ctx)
	if failure != nil {
		return failure
	}
	object, err := client.GetObject(ctx, docName, objName)
	if err != nil {
		Logger.Printf("Failed to get object: %v", err)
		return textResponse("Failed to get object: " + err.Error())
	}
	response := jsonResponse(object)
	screenshot, skip := s.optionalScreenshot(ctx, client, includeScreenshot, viewName)
	return addScreenshotIfAvailable(response, screenshot, skip)
}

// GetPartsList returns the parts library contents.
func (s *State) GetPartsList(ctx context.Context) *mcp.CallToolResult {
	client, failure := s.readyClient(ctx)
	if failure != nil {
		return failure
	}
	parts, err := client.GetPartsList(ctx)
	if err != nil {
		Logger.Printf("Failed to get parts list: %v", err)
		return textResponse("Failed to get parts list: " + err.Error())
	}
	if len(parts) > 0 {
		return jsonResponse(parts)
	}
	return textResponse("No parts found in the parts library. You must add parts_library addon.")
}

// ListDocuments returns the open document names. The Python tool had no guard
// here, so a failure is reported as a tool error.
func (s *State) ListDocuments(ctx context.Context) *mcp.CallToolResult {
	client, failure := s.readyClient(ctx)
	if failure != nil {
		return failure
	}
	documents, err := client.ListDocuments(ctx)
	if err != nil {
		Logger.Printf("Failed to list documents: %v", err)
		return mcp.NewToolResultError(err.Error())
	}
	return jsonResponse(documents)
}

// GetRPCStatus reports RPC and GUI-dispatch health.
func (s *State) GetRPCStatus(ctx context.Context) *mcp.CallToolResult {
	client, failure := s.readyClient(ctx)
	if failure != nil {
		return failure
	}
	status, err := client.GetRPCStatus(ctx)
	if err != nil {
		Logger.Printf("Failed to get RPC status: %v", err)
		return textResponse("Failed to get RPC status: " + err.Error())
	}
	return jsonResponse(status)
}

// ReloadDocument closes and re-opens a document to pick up external edits.
func (s *State) ReloadDocument(ctx context.Context, docName string) *mcp.CallToolResult {
	client, failure := s.readyClient(ctx)
	if failure != nil {
		return failure
	}
	res, err := client.ReloadDocument(ctx, docName)
	if err != nil {
		Logger.Printf("Failed to reload document: %v", err)
		return textResponse("Failed to reload document: " + err.Error())
	}
	if freecad.GetBool(res, "success") {
		return textResponse(fmt.Sprintf("Document '%s' reloaded from disk.", freecad.GetString(res, "document_name")))
	}
	return textResponse("Failed to reload document: " + freecad.GetString(res, "error"))
}

// RunFEMAnalysis runs CalculiX on an existing analysis container.
func (s *State) RunFEMAnalysis(
	ctx context.Context, docName, analysisName string, timeout int,
	includeScreenshot bool, viewName string,
) *mcp.CallToolResult {
	client, failure := s.readyClient(ctx)
	if failure != nil {
		return failure
	}
	res, err := client.RunFEMAnalysis(ctx, docName, analysisName, timeout)
	if err != nil {
		Logger.Printf("Failed to run FEM analysis: %v", err)
		return textResponse("Failed to run FEM analysis: " + err.Error())
	}
	if !freecad.GetBool(res, "success") {
		return jsonResponse(withSummary(
			fmt.Sprintf("FEM analysis '%s' failed: %s", analysisName, pythonValue(res["error"])), res))
	}
	summary := fmt.Sprintf(
		"FEM analysis '%s' solved. max von Mises = %s, max displacement = %s (%s nodes).",
		analysisName,
		formatQuantity(res, "max_von_mises_MPa", "MPa"),
		formatQuantity(res, "max_displacement_mm", "mm"),
		pythonValue(res["node_count"]),
	)
	screenshot, skip := s.optionalScreenshot(ctx, client, includeScreenshot, viewName)
	return addScreenshotIfAvailable(jsonResponse(withSummary(summary, res)), screenshot, skip)
}

// formatHeadlessResult mirrors operations.core.format_headless_result.
func formatHeadlessResult(res map[string]any) string {
	var text string
	success := freecad.GetBool(res, "success")
	if success {
		text = "Headless FreeCAD script finished (exit 0)."
	} else {
		message := freecad.GetString(res, "error")
		if message == "" {
			message = "unknown error"
		}
		text = "Headless FreeCAD script FAILED: " + message
	}
	if output := freecad.GetString(res, "output"); output != "" {
		text += "\nOutput:\n" + strings.TrimRightFunc(output, unicode.IsSpace)
	}
	if success {
		text += "\nIf the script saved a document that is open in the GUI, call reload_document to see the result."
	}
	return text
}

// formatQuantity renders a solver result, or the placeholder the Python client
// used when the addon could not compute it.
func formatQuantity(members map[string]any, key, unit string) string {
	if value, ok := freecad.GetNumber(members, key); ok {
		return strconv.FormatFloat(value, 'g', 4, 64) + " " + unit
	}
	return "unavailable (" + unit + ")"
}

// withSummary puts the human-readable summary alongside the raw solver result,
// the way the Python client merged {"summary": ...} with the response fields.
func withSummary(summary string, res map[string]any) map[string]any {
	payload := make(map[string]any, len(res)+1)
	payload["summary"] = summary
	for key, value := range res {
		payload[key] = value
	}
	return payload
}

// pythonValue renders a value the way Python's str() does inside an f-string.
func pythonValue(value any) string {
	if value == nil {
		return "None"
	}
	return fmt.Sprintf("%v", value)
}

func orEmptyMap(values map[string]any) map[string]any {
	if values == nil {
		return map[string]any{}
	}
	return values
}
