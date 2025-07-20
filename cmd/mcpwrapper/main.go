package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

type MCPWrapper struct {
	server         *server.MCPServer
	binaryPath     string
	serverArgs     []string
	currentProcess *exec.Cmd
	currentStdin   io.WriteCloser
	currentStdout  io.ReadCloser
	watcher        *fsnotify.Watcher
	mu             sync.RWMutex
	isRestarting   bool
	currentTools   map[string]*mcp.Tool
	requestID      int
	logFile        *os.File
}

type MCPMessage struct {
	JSONRPC string      `json:"jsonrpc"`
	Method  string      `json:"method,omitempty"`
	Params  interface{} `json:"params,omitempty"`
	Result  interface{} `json:"result,omitempty"`
	Error   interface{} `json:"error,omitempty"`
	ID      interface{} `json:"id,omitempty"`
}

func NewMCPWrapper(binaryPath string, serverArgs ...string) (*MCPWrapper, error) {
	absPath, err := filepath.Abs(binaryPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path: %w", err)
	}

	wrapper := &MCPWrapper{
		binaryPath:   absPath,
		serverArgs:   serverArgs,
		currentTools: make(map[string]*mcp.Tool),
	}

	// Set up logging if MCPWRAPPER_LOG_FILE is set
	if logPath := os.Getenv("MCPWRAPPER_LOG_FILE"); logPath != "" {
		logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return nil, fmt.Errorf("failed to open log file %s: %w", logPath, err)
		}
		wrapper.logFile = logFile
		wrapper.logEvent("WRAPPER_START", "MCP Wrapper started", map[string]interface{}{
			"binary_path": absPath,
			"server_args": serverArgs,
		})
	}

	// Create the wrapper MCP server
	wrapper.server = server.NewMCPServer("mcpwrapper", "1.0.0")

	// Set up file watcher
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create watcher: %w", err)
	}
	wrapper.watcher = watcher

	// Add the binary file to watcher
	if err := watcher.Add(absPath); err != nil {
		return nil, fmt.Errorf("failed to watch binary: %w", err)
	}

	return wrapper, nil
}

func (w *MCPWrapper) Start() error {
	// Start watching for file changes
	go w.watchFileChanges()

	// Start the underlying server initially
	if err := w.startUnderlyingServer(); err != nil {
		return fmt.Errorf("failed to start underlying server: %w", err)
	}

	// Load initial tools
	if err := w.loadToolsFromServer(); err != nil {
		log.Printf("Warning: failed to load initial tools: %v", err)
	}

	// Start the wrapper MCP server
	return server.ServeStdio(w.server)
}

func (w *MCPWrapper) watchFileChanges() {
	for {
		select {
		case event, ok := <-w.watcher.Events:
			if !ok {
				return
			}

			if event.Has(fsnotify.Write) || event.Has(fsnotify.Create) {
				log.Printf("Binary changed: %s", event.Name)
				w.logEvent("BINARY_CHANGED", "Detected binary file change", map[string]interface{}{
					"file_path": event.Name,
					"operation": event.Op.String(),
				})

				// Small delay to ensure write is complete
				time.Sleep(100 * time.Millisecond)

				if err := w.restartServer(); err != nil {
					log.Printf("Failed to restart server: %v", err)
					w.logEvent("RESTART_FAILED", "Server restart failed", map[string]interface{}{
						"error": err.Error(),
					})
				}
			}

		case err, ok := <-w.watcher.Errors:
			if !ok {
				return
			}
			log.Printf("Watcher error: %v", err)
			w.logEvent("WATCHER_ERROR", "File watcher error", map[string]interface{}{
				"error": err.Error(),
			})
		}
	}
}

func (w *MCPWrapper) startUnderlyingServer() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	cmd := exec.Command(w.binaryPath, w.serverArgs...)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start server: %w", err)
	}

	w.currentProcess = cmd
	w.currentStdin = stdin
	w.currentStdout = stdout

	log.Printf("Started underlying server: PID %d", cmd.Process.Pid)
	return nil
}

func (w *MCPWrapper) stopUnderlyingServer() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.currentProcess == nil {
		return nil
	}

	// Close pipes
	if w.currentStdin != nil {
		w.currentStdin.Close()
	}
	if w.currentStdout != nil {
		w.currentStdout.Close()
	}

	// Kill process
	if err := w.currentProcess.Process.Kill(); err != nil {
		log.Printf("Warning: failed to kill process: %v", err)
	}

	// Wait for it to exit
	_ = w.currentProcess.Wait()
	w.currentProcess = nil
	w.currentStdin = nil
	w.currentStdout = nil

	log.Printf("Stopped underlying server")
	return nil
}

func (w *MCPWrapper) restartServer() error {
	w.mu.Lock()
	w.isRestarting = true
	w.mu.Unlock()

	defer func() {
		w.mu.Lock()
		w.isRestarting = false
		w.mu.Unlock()
	}()

	log.Printf("Restarting server due to binary change...")
	w.logEvent("SERVER_RESTART_START", "Server restart initiated due to binary change", nil)

	// Remove all current tools
	w.removeAllTools()

	// Stop current server
	if err := w.stopUnderlyingServer(); err != nil {
		w.logEvent("SERVER_RESTART_ERROR", "Failed to stop server during restart", map[string]interface{}{
			"error": err.Error(),
		})
		return fmt.Errorf("failed to stop server: %w", err)
	}

	// Start new server
	if err := w.startUnderlyingServer(); err != nil {
		w.logEvent("SERVER_RESTART_ERROR", "Failed to start new server during restart", map[string]interface{}{
			"error": err.Error(),
		})
		return fmt.Errorf("failed to start new server: %w", err)
	}

	// Load new tools
	if err := w.loadToolsFromServer(); err != nil {
		w.logEvent("SERVER_RESTART_ERROR", "Failed to load tools during restart", map[string]interface{}{
			"error": err.Error(),
		})
		return fmt.Errorf("failed to load new tools: %w", err)
	}

	log.Printf("Server restart completed successfully")
	w.logEvent("SERVER_RESTART_COMPLETE", "Server restart completed successfully", nil)
	return nil
}

func (w *MCPWrapper) removeAllTools() {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Get list of current tool names
	var toolNames []string
	for name := range w.currentTools {
		toolNames = append(toolNames, name)
	}

	if len(toolNames) > 0 {
		log.Printf("Removing %d tools: %v", len(toolNames), toolNames)
		w.logEvent("TOOLS_REMOVED", fmt.Sprintf("Removed %d tools", len(toolNames)), map[string]interface{}{
			"count":      len(toolNames),
			"tool_names": toolNames,
		})
		w.server.DeleteTools(toolNames...)
		w.currentTools = make(map[string]*mcp.Tool)
	}
}

func (w *MCPWrapper) loadToolsFromServer() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Initialize the underlying server
	initReq := MCPMessage{
		JSONRPC: "2.0",
		Method:  "initialize",
		Params: map[string]interface{}{
			"capabilities": map[string]interface{}{},
			"clientInfo": map[string]interface{}{
				"name":    "mcpwrapper",
				"version": "1.0.0",
			},
		},
		ID: w.getNextRequestID(),
	}

	if err := w.sendToServer(initReq); err != nil {
		return fmt.Errorf("failed to send initialize: %w", err)
	}

	// Read initialize response
	if _, err := w.readFromServer(); err != nil {
		return fmt.Errorf("failed to read initialize response: %w", err)
	}

	// List tools
	listReq := MCPMessage{
		JSONRPC: "2.0",
		Method:  "tools/list",
		ID:      w.getNextRequestID(),
	}

	if err := w.sendToServer(listReq); err != nil {
		return fmt.Errorf("failed to send tools/list: %w", err)
	}

	// Read tools/list response
	resp, err := w.readFromServer()
	if err != nil {
		return fmt.Errorf("failed to read tools/list response: %w", err)
	}

	// Parse tools from response
	if err := w.parseAndAddTools(resp); err != nil {
		return fmt.Errorf("failed to parse tools: %w", err)
	}

	return nil
}

func (w *MCPWrapper) sendToServer(msg MCPMessage) error {
	if w.currentStdin == nil {
		return fmt.Errorf("server not running")
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	if _, err := w.currentStdin.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("failed to write to server: %w", err)
	}

	return nil
}

func (w *MCPWrapper) readFromServer() (*MCPMessage, error) {
	if w.currentStdout == nil {
		return nil, fmt.Errorf("server not running")
	}

	reader := bufio.NewReader(w.currentStdout)
	line, err := reader.ReadBytes('\n')
	if err != nil {
		return nil, fmt.Errorf("failed to read from server: %w", err)
	}

	var msg MCPMessage
	if err := json.Unmarshal(line, &msg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal message: %w", err)
	}

	return &msg, nil
}

func (w *MCPWrapper) parseAndAddTools(resp *MCPMessage) error {
	if resp.Result == nil {
		return fmt.Errorf("no result in tools/list response")
	}

	result, ok := resp.Result.(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid result format")
	}

	toolsData, ok := result["tools"].([]interface{})
	if !ok {
		return fmt.Errorf("no tools in result")
	}

	log.Printf("Loading %d tools from server", len(toolsData))
	w.logEvent("TOOLS_LOADING", fmt.Sprintf("Loading %d tools from server", len(toolsData)), map[string]interface{}{
		"count": len(toolsData),
	})

	var addedTools []string
	for _, toolData := range toolsData {
		toolMap, ok := toolData.(map[string]interface{})
		if !ok {
			continue
		}

		// Extract tool information
		name, _ := toolMap["name"].(string)
		description, _ := toolMap["description"].(string)

		if name == "" {
			continue
		}

		// Create tool for wrapper with full schema
		toolOptions := []mcp.ToolOption{mcp.WithDescription(description)}

		// Preserve the original input schema if available
		if inputSchema, ok := toolMap["inputSchema"].(map[string]interface{}); ok {
			// Add parameters from the original schema
			if properties, ok := inputSchema["properties"].(map[string]interface{}); ok {
				if required, ok := inputSchema["required"].([]interface{}); ok {
					requiredSet := make(map[string]bool)
					for _, r := range required {
						if reqStr, ok := r.(string); ok {
							requiredSet[reqStr] = true
						}
					}

					for paramName, paramDef := range properties {
						if paramMap, ok := paramDef.(map[string]interface{}); ok {
							paramType, _ := paramMap["type"].(string)
							paramDesc, _ := paramMap["description"].(string)
							isRequired := requiredSet[paramName]

							switch paramType {
							case "string":
								opts := []mcp.PropertyOption{mcp.Description(paramDesc)}
								if isRequired {
									opts = append(opts, mcp.Required())
								}
								toolOptions = append(toolOptions, mcp.WithString(paramName, opts...))
							case "boolean":
								opts := []mcp.PropertyOption{mcp.Description(paramDesc)}
								if isRequired {
									opts = append(opts, mcp.Required())
								}
								toolOptions = append(toolOptions, mcp.WithBoolean(paramName, opts...))
							case "number":
								opts := []mcp.PropertyOption{mcp.Description(paramDesc)}
								if isRequired {
									opts = append(opts, mcp.Required())
								}
								toolOptions = append(toolOptions, mcp.WithNumber(paramName, opts...))
							case "array":
								opts := []mcp.PropertyOption{mcp.Description(paramDesc)}
								if isRequired {
									opts = append(opts, mcp.Required())
								}
								toolOptions = append(toolOptions, mcp.WithArray(paramName, opts...))
							}
						}
					}
				}
			}
		}

		tool := mcp.NewTool(name, toolOptions...)

		// Create handler that proxies to underlying server
		handler := w.createProxyHandler(name)

		// Add tool to wrapper
		w.server.AddTool(tool, handler)
		w.currentTools[name] = &tool
		addedTools = append(addedTools, name)

		log.Printf("Added tool: %s", name)
	}

	if len(addedTools) > 0 {
		w.logEvent("TOOLS_ADDED", fmt.Sprintf("Added %d tools", len(addedTools)), map[string]interface{}{
			"count":      len(addedTools),
			"tool_names": addedTools,
		})
	}

	return nil
}

func (w *MCPWrapper) createProxyHandler(toolName string) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Log the tool call
		args := req.GetArguments()
		w.logEvent("TOOL_CALL", fmt.Sprintf("Tool '%s' called", toolName), map[string]interface{}{
			"tool_name": toolName,
			"arguments": args,
		})

		w.mu.RLock()
		isRestarting := w.isRestarting
		w.mu.RUnlock()

		if isRestarting {
			result := &mcp.CallToolResult{
				Content: []mcp.Content{
					mcp.TextContent{
						Type: "text",
						Text: "Server is restarting, please try again in a moment",
					},
				},
				IsError: true,
			}
			w.logEvent("TOOL_RESULT", fmt.Sprintf("Tool '%s' failed (server restarting)", toolName), map[string]interface{}{
				"tool_name": toolName,
				"error":     true,
			})
			return result, nil
		}

		// Forward request to underlying server
		forwardReq := MCPMessage{
			JSONRPC: "2.0",
			Method:  "tools/call",
			Params: map[string]interface{}{
				"name":      toolName,
				"arguments": req.GetArguments(),
			},
			ID: w.getNextRequestID(),
		}

		w.mu.RLock()
		defer w.mu.RUnlock()

		if err := w.sendToServer(forwardReq); err != nil {
			result := &mcp.CallToolResult{
				Content: []mcp.Content{
					mcp.TextContent{
						Type: "text",
						Text: fmt.Sprintf("Failed to forward request: %v", err),
					},
				},
				IsError: true,
			}
			w.logEvent("TOOL_RESULT", fmt.Sprintf("Tool '%s' failed (forward error)", toolName), map[string]interface{}{
				"tool_name": toolName,
				"error":     true,
				"reason":    err.Error(),
			})
			return result, nil
		}

		// Read response
		resp, err := w.readFromServer()
		if err != nil {
			result := &mcp.CallToolResult{
				Content: []mcp.Content{
					mcp.TextContent{
						Type: "text",
						Text: fmt.Sprintf("Failed to read response: %v", err),
					},
				},
				IsError: true,
			}
			w.logEvent("TOOL_RESULT", fmt.Sprintf("Tool '%s' failed (read error)", toolName), map[string]interface{}{
				"tool_name": toolName,
				"error":     true,
				"reason":    err.Error(),
			})
			return result, nil
		}

		// Convert response to CallToolResult
		result, convErr := w.convertToCallToolResult(resp)

		// Log the result
		if result.IsError {
			w.logEvent("TOOL_RESULT", fmt.Sprintf("Tool '%s' completed with error", toolName), map[string]interface{}{
				"tool_name": toolName,
				"error":     true,
			})
		} else {
			w.logEvent("TOOL_RESULT", fmt.Sprintf("Tool '%s' completed successfully", toolName), map[string]interface{}{
				"tool_name": toolName,
				"error":     false,
			})
		}

		return result, convErr
	}
}

func (w *MCPWrapper) convertToCallToolResult(resp *MCPMessage) (*mcp.CallToolResult, error) {
	if resp.Error != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: fmt.Sprintf("Tool error: %v", resp.Error),
				},
			},
			IsError: true,
		}, nil
	}

	if resp.Result == nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: "No result from tool",
				},
			},
		}, nil
	}

	result, ok := resp.Result.(map[string]interface{})
	if !ok {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: fmt.Sprintf("Invalid result format: %v", resp.Result),
				},
			},
		}, nil
	}

	// Extract content
	var content []mcp.Content
	if contentData, ok := result["content"].([]interface{}); ok {
		for _, item := range contentData {
			if itemMap, ok := item.(map[string]interface{}); ok {
				if text, ok := itemMap["text"].(string); ok {
					content = append(content, mcp.TextContent{
						Type: "text",
						Text: text,
					})
				}
			}
		}
	}

	isError := false
	if errFlag, ok := result["isError"].(bool); ok {
		isError = errFlag
	}

	return &mcp.CallToolResult{
		Content: content,
		IsError: isError,
	}, nil
}

func (w *MCPWrapper) getNextRequestID() int {
	w.requestID++
	return w.requestID
}

func (w *MCPWrapper) logEvent(eventType, message string, details map[string]interface{}) {
	if w.logFile == nil {
		return
	}

	timestamp := time.Now().Format("2006-01-02 15:04:05.000")
	logEntry := fmt.Sprintf("[%s] %s: %s", timestamp, eventType, message)

	if details != nil {
		var detailStrings []string
		for key, value := range details {
			detailStrings = append(detailStrings, fmt.Sprintf("%s=%v", key, value))
		}
		if len(detailStrings) > 0 {
			logEntry += fmt.Sprintf(" (%s)", strings.Join(detailStrings, ", "))
		}
	}

	logEntry += "\n"
	_, _ = w.logFile.WriteString(logEntry)
	_ = w.logFile.Sync() // Ensure it's written immediately
}

func (w *MCPWrapper) Close() error {
	if w.logFile != nil {
		w.logEvent("WRAPPER_STOP", "MCP Wrapper stopping", nil)
		w.logFile.Close()
	}
	if w.watcher != nil {
		w.watcher.Close()
	}
	return w.stopUnderlyingServer()
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s <mcp-server-binary> [args...]\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "\nThis wrapper monitors the MCP server binary for changes and automatically\n")
		fmt.Fprintf(os.Stderr, "restarts it, updating the tool list dynamically.\n")
		fmt.Fprintf(os.Stderr, "\nEnvironment Variables:\n")
		fmt.Fprintf(os.Stderr, "  MCPWRAPPER_LOG_FILE    Path to log file for detailed human-readable logging\n")
		fmt.Fprintf(os.Stderr, "\nExample:\n")
		fmt.Fprintf(os.Stderr, "  %s ./tmux-mcp\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  MCPWRAPPER_LOG_FILE=/tmp/wrapper.log %s ./tmux-mcp\n", os.Args[0])
		os.Exit(1)
	}

	binaryPath := os.Args[1]
	serverArgs := os.Args[2:]

	wrapper, err := NewMCPWrapper(binaryPath, serverArgs...)
	if err != nil {
		log.Fatalf("Failed to create wrapper: %v", err)
	}
	defer wrapper.Close()

	log.Printf("Starting MCP wrapper for: %s %v", binaryPath, serverArgs)

	if err := wrapper.Start(); err != nil {
		log.Fatalf("Wrapper failed: %v", err)
	}
}
