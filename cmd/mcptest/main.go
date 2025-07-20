package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

type MCPRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
	ID      interface{} `json:"id"`
}

type MCPResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id,omitempty"`
	Result  interface{} `json:"result,omitempty"`
	Error   interface{} `json:"error,omitempty"`
}

type ToolCall struct {
	Tool string
	Args map[string]interface{}
}

type MCPTester struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser
	stderr io.ReadCloser
	mu     sync.Mutex
	nextID int
}

func NewMCPTester(serverCommand string, serverArgs ...string) (*MCPTester, error) {
	cmd := exec.Command(serverCommand, serverArgs...)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start server: %w", err)
	}

	tester := &MCPTester{
		cmd:    cmd,
		stdin:  stdin,
		stdout: stdout,
		stderr: stderr,
		nextID: 1,
	}

	// Start stderr reader
	go tester.readStderr()

	return tester, nil
}

func (m *MCPTester) readStderr() {
	scanner := bufio.NewScanner(m.stderr)
	for scanner.Scan() {
		fmt.Fprintf(os.Stderr, "[SERVER STDERR] %s\n", scanner.Text())
	}
}

func (m *MCPTester) getNextID() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	id := m.nextID
	m.nextID++
	return id
}

func (m *MCPTester) sendRequest(method string, params interface{}) (*MCPResponse, error) {
	req := MCPRequest{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
		ID:      m.getNextID(),
	}

	reqBytes, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	fmt.Printf("→ %s\n", string(reqBytes))

	if _, err := m.stdin.Write(append(reqBytes, '\n')); err != nil {
		return nil, fmt.Errorf("failed to write request: %w", err)
	}

	// Read response
	reader := bufio.NewReader(m.stdout)
	line, err := reader.ReadBytes('\n')
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	fmt.Printf("← %s\n", string(line))

	var resp MCPResponse
	if err := json.Unmarshal(line, &resp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &resp, nil
}

func (m *MCPTester) Initialize() error {
	params := map[string]interface{}{
		"capabilities": map[string]interface{}{},
		"clientInfo": map[string]interface{}{
			"name":    "mcptest",
			"version": "1.0.0",
		},
	}

	resp, err := m.sendRequest("initialize", params)
	if err != nil {
		return fmt.Errorf("initialization failed: %w", err)
	}

	if resp.Error != nil {
		return fmt.Errorf("initialization error: %v", resp.Error)
	}

	fmt.Println("✓ Server initialized successfully")
	return nil
}

func (m *MCPTester) ListTools() error {
	resp, err := m.sendRequest("tools/list", nil)
	if err != nil {
		return fmt.Errorf("failed to list tools: %w", err)
	}

	if resp.Error != nil {
		return fmt.Errorf("tools/list error: %v", resp.Error)
	}

	// Pretty print tools
	if result, ok := resp.Result.(map[string]interface{}); ok {
		if tools, ok := result["tools"].([]interface{}); ok {
			fmt.Printf("\n📋 Available Tools (%d):\n", len(tools))
			for i, tool := range tools {
				if t, ok := tool.(map[string]interface{}); ok {
					name := t["name"]
					desc := t["description"]
					fmt.Printf("  %d. %s - %s\n", i+1, name, desc)
				}
			}
			fmt.Println()
		}
	}

	return nil
}

func (m *MCPTester) CallTool(toolCall ToolCall) error {
	params := map[string]interface{}{
		"name":      toolCall.Tool,
		"arguments": toolCall.Args,
	}

	fmt.Printf("🔧 Calling tool: %s\n", toolCall.Tool)
	if len(toolCall.Args) > 0 {
		fmt.Println("   Arguments:")
		for k, v := range toolCall.Args {
			fmt.Printf("     %s: %v\n", k, v)
		}
	}

	resp, err := m.sendRequest("tools/call", params)
	if err != nil {
		return fmt.Errorf("failed to call tool %s: %w", toolCall.Tool, err)
	}

	if resp.Error != nil {
		fmt.Printf("❌ Tool error: %v\n\n", resp.Error)
		return nil
	}

	// Pretty print result
	if result, ok := resp.Result.(map[string]interface{}); ok {
		isError := false
		if errFlag, ok := result["isError"].(bool); ok {
			isError = errFlag
		}

		if isError {
			fmt.Printf("❌ Tool returned error:\n")
		} else {
			fmt.Printf("✅ Tool result:\n")
		}

		if content, ok := result["content"].([]interface{}); ok {
			for _, item := range content {
				if c, ok := item.(map[string]interface{}); ok {
					if text, ok := c["text"].(string); ok {
						// Indent the output
						lines := strings.Split(text, "\n")
						for _, line := range lines {
							fmt.Printf("   %s\n", line)
						}
					}
				}
			}
		}
	}

	fmt.Println()
	return nil
}

func (m *MCPTester) Close() error {
	m.stdin.Close()
	m.stdout.Close()
	m.stderr.Close()
	return m.cmd.Wait()
}

// Parse tool calls from simple text format
func parseToolCalls(filename string) ([]ToolCall, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	var calls []ToolCall
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Parse format: tool_name arg1=value1 arg2=value2
		parts := strings.Fields(line)
		if len(parts) == 0 {
			continue
		}

		call := ToolCall{
			Tool: parts[0],
			Args: make(map[string]interface{}),
		}

		for _, part := range parts[1:] {
			if strings.Contains(part, "=") {
				kv := strings.SplitN(part, "=", 2)
				key := kv[0]
				value := kv[1]

				// Try to parse as different types
				if value == "true" {
					call.Args[key] = true
				} else if value == "false" {
					call.Args[key] = false
				} else if num, err := strconv.ParseFloat(value, 64); err == nil {
					call.Args[key] = num
				} else if strings.HasPrefix(value, "[") && strings.HasSuffix(value, "]") {
					// Simple array parsing
					value = strings.Trim(value, "[]")
					if value != "" {
						items := strings.Split(value, ",")
						var array []string
						for _, item := range items {
							array = append(array, strings.TrimSpace(item))
						}
						call.Args[key] = array
					} else {
						call.Args[key] = []string{}
					}
				} else {
					call.Args[key] = value
				}
			}
		}

		calls = append(calls, call)
	}

	return calls, scanner.Err()
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s <server-command> [test-file]\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  %s ./tmux-mcp\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s ./tmux-mcp test-calls.txt\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "\nTest file format (one tool call per line):\n")
		fmt.Fprintf(os.Stderr, "  tool_name arg1=value1 arg2=value2\n")
		fmt.Fprintf(os.Stderr, "  tmux_list\n")
		fmt.Fprintf(os.Stderr, "  tmux_new_session command=[echo,hello] prefix=test\n")
		os.Exit(1)
	}

	serverCommand := os.Args[1]
	var testFile string
	if len(os.Args) > 2 {
		testFile = os.Args[2]
	}

	fmt.Printf("🚀 Starting MCP server: %s\n", serverCommand)

	tester, err := NewMCPTester(serverCommand)
	if err != nil {
		log.Fatalf("Failed to start MCP tester: %v", err)
	}
	defer tester.Close()

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	// Initialize
	if err := tester.Initialize(); err != nil {
		log.Fatalf("Failed to initialize: %v", err)
	}

	// List tools
	if err := tester.ListTools(); err != nil {
		log.Fatalf("Failed to list tools: %v", err)
	}

	if testFile != "" {
		// Run test file
		fmt.Printf("📝 Running test file: %s\n\n", testFile)
		calls, err := parseToolCalls(testFile)
		if err != nil {
			log.Fatalf("Failed to parse test file: %v", err)
		}

		for i, call := range calls {
			fmt.Printf("--- Test %d ---\n", i+1)
			if err := tester.CallTool(call); err != nil {
				log.Printf("Test %d failed: %v", i+1, err)
			}
		}

		fmt.Printf("✅ Completed %d test calls\n", len(calls))
	} else {
		// Interactive mode
		fmt.Println("💬 Interactive mode - enter tool calls (Ctrl+C to exit)")
		fmt.Println("Format: tool_name arg1=value1 arg2=value2")
		fmt.Println()

		scanner := bufio.NewScanner(os.Stdin)
		for {
			fmt.Print("> ")
			if !scanner.Scan() {
				break
			}

			line := strings.TrimSpace(scanner.Text())
			if line == "" {
				continue
			}

			if line == "quit" || line == "exit" {
				break
			}

			// Parse the line directly
			parts := strings.Fields(line)
			if len(parts) == 0 {
				continue
			}

			call := ToolCall{
				Tool: parts[0],
				Args: make(map[string]interface{}),
			}

			for _, part := range parts[1:] {
				if strings.Contains(part, "=") {
					kv := strings.SplitN(part, "=", 2)
					key := kv[0]
					value := kv[1]

					// Try to parse as different types
					if value == "true" {
						call.Args[key] = true
					} else if value == "false" {
						call.Args[key] = false
					} else if num, err := strconv.ParseFloat(value, 64); err == nil {
						call.Args[key] = num
					} else if strings.HasPrefix(value, "[") && strings.HasSuffix(value, "]") {
						// Simple array parsing
						value = strings.Trim(value, "[]")
						if value != "" {
							items := strings.Split(value, ",")
							var array []string
							for _, item := range items {
								array = append(array, strings.TrimSpace(item))
							}
							call.Args[key] = array
						} else {
							call.Args[key] = []string{}
						}
					} else {
						call.Args[key] = value
					}
				}
			}

			if err := tester.CallTool(call); err != nil {
				fmt.Printf("Error: %v\n", err)
			}
		}
	}
}
