package tools

import (
	"context"
	"errors"
	"log"
	"os"
	"sync"
	"time"

	"github.com/cnPauLi/freecad-go-mcp/internal/freecad"
)

// Logger writes to stderr: stdout carries the MCP stdio protocol.
var Logger = log.New(os.Stderr, "FreeCADMCPserver - ", log.LstdFlags|log.Lmsgprefix)

// State holds the process-wide configuration and the persistent FreeCAD
// connection, mirroring freecad_mcp.server_state.ServerState.
type State struct {
	OnlyTextFeedback bool
	Host             string
	// FreeCADCmd is the command that starts headless FreeCAD; empty means
	// auto-detect on first use.
	FreeCADCmd []string

	mu     sync.Mutex
	client *freecad.Client

	// port overrides DefaultRPCPort; tests point it at a local server.
	port int
}

// NewState returns a state with the Python defaults.
func NewState() *State {
	return &State{Host: "localhost"}
}

var errNotConnected = errors.New("Failed to connect to FreeCAD. Make sure the FreeCAD addon is running.")

// Connection returns the persistent FreeCAD connection, creating and pinging it
// on first use.
func (s *State) Connection(ctx context.Context) (*freecad.Client, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.client != nil {
		return s.client, nil
	}
	port := s.port
	if port == 0 {
		port = freecad.DefaultRPCPort
	}
	client := freecad.NewClient(s.Host, port, freecad.DefaultTimeout)
	pingCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	ok, err := client.Ping(pingCtx)
	if err != nil || !ok {
		if err != nil {
			Logger.Printf("Failed to ping FreeCAD: %v", err)
		} else {
			Logger.Printf("Failed to ping FreeCAD")
		}
		client.Disconnect()
		return nil, errNotConnected
	}
	s.client = client
	return s.client, nil
}

// Close releases the FreeCAD connection.
func (s *State) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.client != nil {
		s.client.Disconnect()
		s.client = nil
	}
}
