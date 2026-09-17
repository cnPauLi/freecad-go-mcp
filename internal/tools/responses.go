package tools

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
)

// textResponse returns a text-only tool result.
func textResponse(message string) *mcp.CallToolResult {
	return mcp.NewToolResultText(message)
}

// jsonResponse renders data the way the Python client did
// (json.dumps(..., ensure_ascii=False, indent=2, default=str)): UTF-8 is kept
// as-is and HTML characters are not escaped.
func jsonResponse(data any) *mcp.CallToolResult {
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(data); err != nil {
		// Mirrors json.dumps(default=str) for values that cannot be encoded.
		return textResponse(fmt.Sprintf("%v", data))
	}
	return textResponse(strings.TrimRight(buffer.String(), "\n"))
}

// addScreenshotIfAvailable appends a screenshot unless feedback is text-only or
// no screenshot was produced.
func addScreenshotIfAvailable(response *mcp.CallToolResult, screenshot string, skip bool) *mcp.CallToolResult {
	if skip || screenshot == "" {
		return response
	}
	response.Content = append(response.Content, mcp.ImageContent{
		Type:     mcp.ContentTypeImage,
		Data:     screenshot,
		MIMEType: "image/png",
	})
	return response
}
