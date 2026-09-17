// Command freecad-go-mcp is a Go rewrite of the FreeCAD MCP client. It serves
// the same tools and prompt as the Python package over MCP stdio, forwarding
// calls to the FreeCAD addon's XML-RPC server.
//
// The addon (addon/FreeCADMCP) is unchanged: install it in FreeCAD, start its
// RPC server, then point your MCP client at this binary.
package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"os"
	"regexp"
	"strings"

	"github.com/mark3labs/mcp-go/server"

	"github.com/cnPauLi/freecad-go-mcp/internal/freecad"
	"github.com/cnPauLi/freecad-go-mcp/internal/tools"
)

// version mirrors the Python package version. Release builds override it with
// -ldflags "-X main.version=..."; see the Makefile.
var version = "0.1.23"

func main() {
	onlyTextFeedback := flag.Bool("only-text-feedback", false, "Only return text feedback")
	host := flag.String("host", "localhost", "Host address of the FreeCAD RPC server to connect to (default: localhost)")
	freecadcmd := flag.String("freecadcmd", "", "Command that starts headless FreeCAD for execute_code_headless, e.g. 'flatpak run --command=freecadcmd org.freecad.FreeCAD' (default: auto-detect PATH, then Flatpak)")
	flag.Parse()

	if err := validateHost(*host); err != nil {
		fmt.Fprintf(os.Stderr, "freecad-go-mcp: %v\n", err)
		flag.Usage()
		os.Exit(2)
	}
	command, err := freecad.ParseCommand(*freecadcmd)
	if err != nil {
		fmt.Fprintf(os.Stderr, "freecad-go-mcp: invalid --freecadcmd: %v\n", err)
		os.Exit(2)
	}

	state := tools.NewState()
	state.OnlyTextFeedback = *onlyTextFeedback
	state.Host = *host
	state.FreeCADCmd = command

	tools.Logger.Printf("FreeCADMCP server starting up")
	if _, err := state.Connection(context.Background()); err != nil {
		tools.Logger.Printf("Could not connect to FreeCAD on startup: %v", err)
		tools.Logger.Printf("Make sure the FreeCAD addon is running before using FreeCAD resources or tools")
	} else {
		tools.Logger.Printf("Successfully connected to FreeCAD on startup")
	}
	defer func() {
		tools.Logger.Printf("Disconnecting from FreeCAD on shutdown")
		state.Close()
		tools.Logger.Printf("FreeCADMCP server shut down")
	}()
	tools.Logger.Printf("Only text feedback: %v", state.OnlyTextFeedback)
	tools.Logger.Printf("Connecting to FreeCAD RPC server at: %s", state.Host)

	mcpServer := server.NewMCPServer(
		"FreeCADMCP",
		version,
		server.WithInstructions("FreeCAD integration through the Model Context Protocol"),
		server.WithToolCapabilities(false),
		server.WithPromptCapabilities(false),
		server.WithRecovery(),
	)
	tools.Register(mcpServer, state)

	if err := server.ServeStdio(mcpServer, server.WithErrorLogger(tools.Logger)); err != nil {
		tools.Logger.Fatalf("Server error: %v", err)
	}
}

// hostnameLabel matches one DNS label: letters, digits and inner hyphens.
var hostnameLabel = regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?$`)

// validateHost accepts a valid IPv4/IPv6 address or hostname, mirroring the
// validators.ipv4/ipv6/hostname check of the Python CLI.
func validateHost(value string) error {
	if net.ParseIP(value) != nil || isHostname(value) {
		return nil
	}
	return fmt.Errorf("Invalid host: '%s'. Must be a valid IP address or hostname.", value)
}

func isHostname(value string) bool {
	if value == "" || len(value) > 253 {
		return false
	}
	for _, label := range strings.Split(value, ".") {
		if !hostnameLabel.MatchString(label) {
			return false
		}
	}
	return true
}
