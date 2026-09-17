// Package tools exposes the FreeCAD RPC server as MCP tools and one prompt.
package tools

import (
	"context"
	"strconv"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// viewNames are the orientations the screenshot helpers accept.
var viewNames = []string{
	"Isometric", "Front", "Top", "Right", "Back", "Left", "Bottom", "Dimetric", "Trimetric",
}

// Register adds every FreeCAD tool and the asset creation prompt.
func Register(s *server.MCPServer, state *State) {
	registerDocumentTools(s, state)
	registerObjectTools(s, state)
	registerExecutionTools(s, state)
	registerInspectionTools(s, state)
	registerPrompt(s)
}

func registerDocumentTools(s *server.MCPServer, state *State) {
	s.AddTool(mcp.NewTool("create_document",
		mcp.WithDescription(createDocumentDescription),
		mcp.WithString("name",
			mcp.Required(),
			mcp.Description("The name of the document to create."),
		),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		name, err := request.RequireString("name")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return state.CreateDocument(ctx, name), nil
	})

	s.AddTool(mcp.NewTool("list_documents",
		mcp.WithDescription(listDocumentsDescription),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return state.ListDocuments(ctx), nil
	})

	s.AddTool(mcp.NewTool("reload_document",
		mcp.WithDescription(reloadDocumentDescription),
		mcp.WithString("doc_name",
			mcp.Required(),
			mcp.Description("The name of the open document to reload. Must match the name shown by list_documents."),
		),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		docName, err := request.RequireString("doc_name")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return state.ReloadDocument(ctx, docName), nil
	})
}

func registerObjectTools(s *server.MCPServer, state *State) {
	s.AddTool(mcp.NewTool("create_object",
		mcp.WithDescription(createObjectDescription),
		mcp.WithString("doc_name",
			mcp.Required(),
			mcp.Description("The name of the document to create the object in."),
		),
		mcp.WithString("obj_type",
			mcp.Required(),
			mcp.Description("The type of the object to create (e.g. 'Part::Box', 'Part::Cylinder', 'Draft::Circle', 'PartDesign::Body', etc.)."),
		),
		mcp.WithString("obj_name",
			mcp.Required(),
			mcp.Description("The name of the object to create."),
		),
		mcp.WithString("analysis_name",
			mcp.Description("The name of the FEM analysis to add the object to."),
		),
		mcp.WithObject("obj_properties",
			mcp.Description("The properties of the object to create."),
		),
		includeScreenshotOption(),
		viewNameOption(),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		docName, err := request.RequireString("doc_name")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		objType, err := request.RequireString("obj_type")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		objName, err := request.RequireString("obj_name")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return state.CreateObject(
			ctx, docName, objType, objName,
			optionalString(request, "analysis_name"),
			objectArgument(request, "obj_properties"),
			includeScreenshot(request), viewName(request),
		), nil
	})

	s.AddTool(mcp.NewTool("edit_object",
		mcp.WithDescription(editObjectDescription),
		mcp.WithString("doc_name",
			mcp.Required(),
			mcp.Description("The name of the document to edit the object in."),
		),
		mcp.WithString("obj_name",
			mcp.Required(),
			mcp.Description("The name of the object to edit."),
		),
		mcp.WithObject("obj_properties",
			mcp.Required(),
			mcp.Description("The properties of the object to edit."),
		),
		includeScreenshotOption(),
		viewNameOption(),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		docName, err := request.RequireString("doc_name")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		objName, err := request.RequireString("obj_name")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return state.EditObject(
			ctx, docName, objName,
			objectArgument(request, "obj_properties"),
			includeScreenshot(request), viewName(request),
		), nil
	})

	s.AddTool(mcp.NewTool("delete_object",
		mcp.WithDescription(deleteObjectDescription),
		mcp.WithString("doc_name",
			mcp.Required(),
			mcp.Description("The name of the document to delete the object from."),
		),
		mcp.WithString("obj_name",
			mcp.Required(),
			mcp.Description("The name of the object to delete."),
		),
		includeScreenshotOption(),
		viewNameOption(),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		docName, err := request.RequireString("doc_name")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		objName, err := request.RequireString("obj_name")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return state.DeleteObject(ctx, docName, objName, includeScreenshot(request), viewName(request)), nil
	})

	s.AddTool(mcp.NewTool("insert_part_from_library",
		mcp.WithDescription(insertPartFromLibraryDescription),
		mcp.WithString("relative_path",
			mcp.Required(),
			mcp.Description("The relative path of the part to insert."),
		),
		includeScreenshotOption(),
		viewNameOption(),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		relativePath, err := request.RequireString("relative_path")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return state.InsertPartFromLibrary(ctx, relativePath, includeScreenshot(request), viewName(request)), nil
	})

	s.AddTool(mcp.NewTool("get_parts_list",
		mcp.WithDescription(getPartsListDescription),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return state.GetPartsList(ctx), nil
	})
}

func registerExecutionTools(s *server.MCPServer, state *State) {
	s.AddTool(mcp.NewTool("execute_code",
		mcp.WithDescription(executeCodeDescription),
		mcp.WithString("code",
			mcp.Required(),
			mcp.Description("The Python code to execute."),
		),
		includeScreenshotOption(),
		viewNameOption(),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		code, err := request.RequireString("code")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return state.ExecuteCode(ctx, code, includeScreenshot(request), viewName(request)), nil
	})

	s.AddTool(mcp.NewTool("execute_code_async",
		mcp.WithDescription(executeCodeAsyncDescription),
		mcp.WithString("code",
			mcp.Required(),
			mcp.Description("Background-safe Python code to execute. Use commit(fn) for all document and view writes."),
		),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		code, err := request.RequireString("code")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return state.ExecuteCodeAsync(ctx, code), nil
	})

	s.AddTool(mcp.NewTool("get_async_status",
		mcp.WithDescription(getAsyncStatusDescription),
		mcp.WithString("job_id",
			mcp.Description("The id returned by execute_code_async. Empty lists all running jobs and up to 20 recently completed jobs."),
			mcp.DefaultString(""),
		),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return state.GetAsyncStatus(ctx, request.GetString("job_id", "")), nil
	})

	s.AddTool(mcp.NewTool("execute_code_headless",
		mcp.WithDescription(executeCodeHeadlessDescription),
		mcp.WithString("code",
			mcp.Required(),
			mcp.Description("Complete Python script for freecadcmd."),
		),
		mcp.WithNumber("timeout",
			mcp.Description("Positive finite seconds to wait before killing the process (default 600). Partial output is preserved on timeout."),
			mcp.DefaultNumber(600),
		),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		code, err := request.RequireString("code")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return state.ExecuteCodeHeadless(ctx, code, request.GetFloat("timeout", 600)), nil
	})

	s.AddTool(mcp.NewTool("run_fem_analysis",
		mcp.WithDescription(runFEMAnalysisDescription),
		mcp.WithString("doc_name",
			mcp.Required(),
			mcp.Description("Name of the FreeCAD document."),
		),
		mcp.WithString("analysis_name",
			mcp.Required(),
			mcp.Description("Name of the Fem::AnalysisPython object."),
		),
		mcp.WithNumber("timeout",
			mcp.Description("Seconds to wait for the solver (default 600)."),
			mcp.DefaultNumber(600),
		),
		includeScreenshotOption(),
		viewNameOption(),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		docName, err := request.RequireString("doc_name")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		analysisName, err := request.RequireString("analysis_name")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return state.RunFEMAnalysis(
			ctx, docName, analysisName, request.GetInt("timeout", 600),
			includeScreenshot(request), viewName(request),
		), nil
	})
}

func registerInspectionTools(s *server.MCPServer, state *State) {
	s.AddTool(mcp.NewTool("get_view",
		mcp.WithDescription(getViewDescription),
		mcp.WithString("view_name",
			mcp.Required(),
			mcp.Description("The name of the view to get the screenshot of."),
			mcp.Enum(viewNames...),
		),
		mcp.WithNumber("width",
			mcp.Description("The width of the screenshot in pixels. If not specified, uses the viewport width."),
		),
		mcp.WithNumber("height",
			mcp.Description("The height of the screenshot in pixels. If not specified, uses the viewport height."),
		),
		mcp.WithString("focus_object",
			mcp.Description("The name of the object to focus on. If not specified, fits all objects in the view."),
		),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		viewName, err := request.RequireString("view_name")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return state.GetView(
			ctx, viewName,
			optionalInt(request, "width"), optionalInt(request, "height"),
			optionalString(request, "focus_object"),
		), nil
	})

	s.AddTool(mcp.NewTool("get_objects",
		mcp.WithDescription(getObjectsDescription),
		mcp.WithString("doc_name",
			mcp.Required(),
			mcp.Description("The name of the document to get the objects from."),
		),
		includeScreenshotOption(),
		viewNameOption(),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		docName, err := request.RequireString("doc_name")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return state.GetObjects(ctx, docName, includeScreenshot(request), viewName(request)), nil
	})

	s.AddTool(mcp.NewTool("get_object",
		mcp.WithDescription(getObjectDescription),
		mcp.WithString("doc_name",
			mcp.Required(),
			mcp.Description("The name of the document to get the object from."),
		),
		mcp.WithString("obj_name",
			mcp.Required(),
			mcp.Description("The name of the object to get."),
		),
		includeScreenshotOption(),
		viewNameOption(),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		docName, err := request.RequireString("doc_name")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		objName, err := request.RequireString("obj_name")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return state.GetObject(ctx, docName, objName, includeScreenshot(request), viewName(request)), nil
	})

	s.AddTool(mcp.NewTool("get_rpc_status",
		mcp.WithDescription(getRPCStatusDescription),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return state.GetRPCStatus(ctx), nil
	})
}

func registerPrompt(s *server.MCPServer) {
	s.AddPrompt(mcp.NewPrompt("asset_creation_strategy"),
		func(ctx context.Context, request mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
			return &mcp.GetPromptResult{
				Messages: []mcp.PromptMessage{
					{
						Role:    mcp.RoleUser,
						Content: mcp.TextContent{Type: mcp.ContentTypeText, Text: assetCreationStrategy},
					},
				},
			}, nil
		})
}

func includeScreenshotOption() mcp.ToolOption {
	return mcp.WithBoolean("include_screenshot",
		mcp.Description("Whether to return a screenshot of the model (default True)."),
		mcp.DefaultBool(true),
	)
}

func viewNameOption() mcp.ToolOption {
	return mcp.WithString("view_name",
		mcp.Description(`The view orientation of the returned screenshot (default "Isometric").`),
		mcp.Enum(viewNames...),
		mcp.DefaultString("Isometric"),
	)
}

func includeScreenshot(request mcp.CallToolRequest) bool {
	return request.GetBool("include_screenshot", true)
}

func viewName(request mcp.CallToolRequest) string {
	return request.GetString("view_name", "Isometric")
}

// optionalString distinguishes an absent argument from an empty one, so an
// omitted value reaches FreeCAD as None.
func optionalString(request mcp.CallToolRequest, key string) *string {
	value, ok := request.GetArguments()[key]
	if !ok || value == nil {
		return nil
	}
	text, ok := value.(string)
	if !ok {
		return nil
	}
	return &text
}

// optionalInt distinguishes an absent argument from zero.
func optionalInt(request mcp.CallToolRequest, key string) *int {
	value, ok := request.GetArguments()[key]
	if !ok || value == nil {
		return nil
	}
	switch typed := value.(type) {
	case float64:
		number := int(typed)
		return &number
	case int:
		return &typed
	case int64:
		number := int(typed)
		return &number
	case string:
		if number, err := strconv.Atoi(typed); err == nil {
			return &number
		}
	}
	return nil
}

// objectArgument reads an arbitrary JSON object argument.
func objectArgument(request mcp.CallToolRequest, key string) map[string]any {
	value, ok := request.GetArguments()[key]
	if !ok || value == nil {
		return nil
	}
	object, ok := value.(map[string]any)
	if !ok {
		return nil
	}
	return object
}
