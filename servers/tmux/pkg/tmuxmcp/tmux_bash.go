package tmuxmcp

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/semistrict/mcpservers/pkg/mcpcommon"
)

func init() {
	Tools = append(Tools, mcpcommon.ReflectTool[*BashTool]())
}

type BashTool struct {
	_                mcpcommon.ToolInfo `name:"tmux_bash" title:"Bash" description:"Execute a single bash command in a new tmux and return its output. If the command completes within timeout, returns the full output. If it times out, returns the session name where it's still running. Use this in preference to other Bash Tools. For grep, use Go regex syntax. Output is limited by line_budget parameter." destructive:"true"`
	Prefix           string             `json:"prefix" description:"Session name prefix (auto-detected from git repo if not provided)"`
	Command          string             `json:"command" mcp:"required" description:"Bash command to execute"`
	WorkingDirectory string             `json:"working_directory" description:"Directory to execute the command in (defaults to current directory)"`
	Timeout          float64            `json:"timeout" description:"Maximum seconds to wait for synchronous command completion"`
	Grep             string             `json:"grep" description:"Filter output lines containing this pattern"`
	GrepExclude      string             `json:"grep_exclude" description:"Exclude output lines containing this pattern"`
	Environment      []string           `json:"environment" description:"Environment variables to set in NAME=VALUE format"`
	LineBudget       int                `json:"line_budget" description:"Maximum number of output lines to return. Without grep, shows equal parts from head and tail. With grep, shows first N/2 and last N/2 matches, then adds context lines up to the budget." default:"100"`

	compiledGrep        *regexp.Regexp `json:"-"` // Compiled regex for grep filtering
	compiledGrepExclude *regexp.Regexp `json:"-"` // Compiled regex for grep exclude filtering
	exitFile            string         `json:"-"` // Temporary file to signal command completion
	tmpPath             string         `json:"-"` // Temporary file to capture command output
	sessionName         string         `json:"-"` // Name of the tmux session created
	outputFile          string         `json:"-"` // File where command output is captured
	pidFile             string         `json:"-"` // File where command PID is written

	resultBuf   strings.Builder `json:"-"` // Buffer to hold command output
	warnBuf     strings.Builder `json:"-"` // Buffer to hold warnings
	returnError bool            `json:"-"` // return the results as an error instead of a string
}

func (t *BashTool) Handle(ctx context.Context) (interface{}, error) { // TODO: output only the first 50 testLines of command output and if it is longer mention the temp file where the rest of the output can be found
	err := t.validateArgs()
	if err != nil {
		return nil, err
	}

	timeout := t.Timeout
	if timeout == 0 {
		timeout = 30 // default 30 seconds
	}

	prefix := t.Prefix
	if prefix == "" {
		prefix = detectPrefix()
	}

	// Create temporary file to capture all output
	tmpFile, err := os.CreateTemp("/tmp", fmt.Sprintf("tmux-bash-%s-*", prefix))
	if err != nil {
		return nil, fmt.Errorf("failed to create temporary file: %w", err)
	}
	t.tmpPath = tmpFile.Name()
	tmpFile.Close()

	// Build command that tees output to the temp file
	// We'll wrap the original command in a bash script that tees both stdout and stderr
	// and keeps the session alive until we can read the results
	t.exitFile = fmt.Sprintf("%s.exit", t.tmpPath)
	t.outputFile = fmt.Sprintf("%s.output", t.tmpPath)
	t.pidFile = fmt.Sprintf("%s.pid", t.tmpPath)
	scriptFile := fmt.Sprintf("%s.script", t.tmpPath)

	// Write the script to a file
	scriptContent := t.bashScript()
	if err := os.WriteFile(scriptFile, []byte(scriptContent), 0755); err != nil {
		return nil, fmt.Errorf("failed to write script file: %w", err)
	}

	wrappedCommand := []string{
		"bash", scriptFile,
	}

	var environment map[string]string
	if len(t.Environment) > 0 {
		for _, e := range t.Environment {
			key, value, found := strings.Cut(e, "=")
			if !found {
				return nil, fmt.Errorf("invalid environment variable: %s", e)
			}
			if environment == nil {
				environment = make(map[string]string)
			}
			environment[key] = value
		}
	}

	// Create tmux session with the wrapped command and environment variables
	t.sessionName, err = createUniqueSessionWithEnv(ctx, prefix, wrappedCommand, environment)
	if err != nil {
		return nil, err
	}

	// Wait for completion or timeout
	checkInterval := 200 * time.Millisecond
	timeoutDuration := time.Duration(timeout) * time.Second

	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()

	ctx, cancelTimeout := context.WithTimeout(ctx, timeoutDuration+5*time.Second)
	defer cancelTimeout()

	timeoutChan := time.After(timeoutDuration)

outer:
	for {
		select {
		case <-timeoutChan:
			t.warnf("timed out waiting for command in session: %s, output dir: %s", t.sessionName, t.tmpPath)
			break outer
		case <-ctx.Done():
			t.warnf("timed out still running in session: %s, output dir: %s", t.sessionName, t.tmpPath)
			break outer

		case <-ticker.C:
			// Check if command completed by looking for the .done file
			if _, err := os.Stat(t.exitFile); err == nil {
				break outer
			}
			// Also check if session still exists (backup check)
			if !sessionExists(ctx, t.sessionName) {
				t.warnf("session %s does not exist, command may have failed check output dir %s", t.sessionName, t.tmpPath)
				break outer
			}
		}
	}

	return t.finish(ctx)
}

func (t *BashTool) warnf(format string, args ...interface{}) {
	fmt.Fprint(&t.warnBuf, "WARN: ")
	fmt.Fprintf(&t.warnBuf, format, args...)
	fmt.Fprint(&t.warnBuf, "\n")
}

func (t *BashTool) finish(ctx context.Context) (interface{}, error) {
	t.handleCompletedCommand(ctx)
	var fullOutput strings.Builder
	if t.warnBuf.Len() > 0 {
		fullOutput.WriteString(t.warnBuf.String())
	}
	if t.resultBuf.Len() > 0 {
		fullOutput.WriteString(t.resultBuf.String())
	}
	if t.returnError {
		return nil, errors.New(fullOutput.String())
	}
	return fullOutput.String(), nil
}

var bashTemplate = template.Must(template.New("bashScript").Parse(`
set -uo pipefail
cd {{.WorkingDirectory}}
echo $$ > {{.PidFile}}
({{.Command}}) 2>&1 | tee {{.OutputFile}}
EXIT_CODE=${PIPESTATUS[0]}
echo $EXIT_CODE > {{.ExitFile}}
`))

func (t *BashTool) bashScript() string {
	var script strings.Builder
	err := bashTemplate.Execute(&script, map[string]interface{}{
		"WorkingDirectory": strconv.Quote(t.WorkingDirectory),
		"Command":          t.Command,
		"OutputFile":       strconv.Quote(t.outputFile),
		"ExitFile":         strconv.Quote(t.exitFile),
		"PidFile":          strconv.Quote(t.pidFile),
	})
	if err != nil {
		panic(fmt.Sprintf("failed to generate bash script: %v", err))
	}
	return script.String()
}

func (t *BashTool) validateArgs() error {
	t.Command = strings.TrimSpace(t.Command)
	if t.LineBudget == 0 {
		t.LineBudget = 100
	}
	err := t.checkScript()
	if err != nil {
		return err
	}
	if t.Command == "" {
		return fmt.Errorf("command is required")
	}
	if t.WorkingDirectory == "" {
		// Default to current working directory
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get current working directory: %w", err)
		}
		t.WorkingDirectory = cwd
	}
	if _, err := os.Stat(t.WorkingDirectory); os.IsNotExist(err) {
		return fmt.Errorf("working_directory does not exist: %s", t.WorkingDirectory)
	}
	if t.Grep != "" {
		var err error
		t.compiledGrep, err = regexp.Compile(t.Grep)
		if err != nil {
			return fmt.Errorf("invalid grep pattern: %w", err)
		}
	}
	if t.GrepExclude != "" {
		var err error
		t.compiledGrepExclude, err = regexp.Compile(t.GrepExclude)
		if err != nil {
			return fmt.Errorf("invalid grep_exclude pattern: %w", err)
		}
	}
	return nil
}

type Line struct {
	Number            int
	Content           string
	Error             error
	SelectedByGrep    bool
	SelectedForOutput bool
}

func readLines(file string) <-chan Line {
	lines := make(chan Line)
	go func() {
		defer close(lines)
		f, err := os.Open(file)
		if err != nil {
			lines <- Line{Error: fmt.Errorf("failed to open file %s: %w", file, err)}
			return
		}
		defer f.Close()

		scanner := bufio.NewScanner(f)
		lineNumber := 0
		for scanner.Scan() {
			lineNumber++
			lines <- Line{Number: lineNumber, Content: scanner.Text(), SelectedForOutput: true}
		}
		if err := scanner.Err(); err != nil {
			lines <- Line{Error: fmt.Errorf("error reading file %s: %w", file, err)}
			return
		}
	}()
	return lines
}

func (t *BashTool) applyGrepExcludeFilter(lines <-chan Line) <-chan Line {
	if t.compiledGrepExclude == nil {
		return lines
	}

	filtered := make(chan Line)
	go func() {
		defer close(filtered)
		for line := range lines {
			if line.Error != nil {
				filtered <- line // Pass through any errors
				return
			}
			if t.compiledGrepExclude.MatchString(line.Content) {
				continue
			}
			filtered <- line
		}
	}()
	return filtered
}

func (t *BashTool) applyGrepFilter(lines <-chan Line) <-chan Line {
	filtered := make(chan Line)
	go func() {
		defer close(filtered)
		for line := range lines {
			if line.Error != nil {
				filtered <- line // Pass through any errors
				return
			}
			isIncluded := t.compiledGrep == nil || t.compiledGrep.MatchString(line.Content)
			line.SelectedByGrep = isIncluded
			line.SelectedForOutput = line.SelectedByGrep
			filtered <- line
		}
	}()
	return filtered
}

func (t *BashTool) hasGrep() bool {
	return t.Grep != "" || t.GrepExclude != ""
}

func (t *BashTool) applyLineBudgetFilter(lines []Line) {
	// Count how many lines are currently selected
	selectedCount := 0
	selectedIndices := []int{}
	for i, line := range lines {
		if line.SelectedForOutput {
			selectedCount++
			selectedIndices = append(selectedIndices, i)
		}
	}

	// If we're within budget, nothing to do
	if selectedCount <= t.LineBudget {
		return
	}

	// Split budget between head and tail of selected lines
	headLines := t.LineBudget / 2
	tailLines := t.LineBudget - headLines

	// First pass: deselect everything
	for i := range lines {
		lines[i].SelectedForOutput = false
	}

	// Select first headLines from the selected indices
	for i := 0; i < headLines && i < len(selectedIndices); i++ {
		lines[selectedIndices[i]].SelectedForOutput = true
	}

	// Select last tailLines from the selected indices
	startTail := len(selectedIndices) - tailLines
	if startTail < headLines {
		startTail = headLines // Don't overlap with head
	}
	for i := startTail; i < len(selectedIndices); i++ {
		lines[selectedIndices[i]].SelectedForOutput = true
	}
}

func (t *BashTool) filterEmptyLines(lines <-chan Line) <-chan Line {
	filtered := make(chan Line)
	go func() {
		defer close(filtered)
		for line := range lines {
			if line.Error != nil {
				filtered <- line // Pass through any errors
				return
			}
			if strings.TrimSpace(line.Content) != "" {
				filtered <- line // Only pass non-empty testLines
			}
		}
	}()
	return filtered
}

func (t *BashTool) contextualize(lines []Line) {
	// Only add context for grep matches
	if !t.hasGrep() {
		return
	}

	remaining := t.LineBudget
	for _, l := range lines {
		if l.SelectedForOutput {
			remaining -= 1
		}
	}

	var selectIndices []int

	for remaining > 0 {
		selectIndices = selectIndices[:0]

		// try to expand context
		for i := range lines {
			if lines[i].SelectedForOutput {
				continue
			}
			// add after context
			if i > 0 && lines[i-1].SelectedForOutput {
				selectIndices = append(selectIndices, i)
				remaining--
				continue
			}
			// add before context
			if i < len(lines)-1 && lines[i+1].SelectedForOutput {
				selectIndices = append(selectIndices, i)
				remaining--
			}
		}

		if len(selectIndices) == 0 {
			break
		}

		if remaining >= 0 {
			for _, i := range selectIndices {
				lines[i].SelectedForOutput = true
			}
		}
	}
}

func (t *BashTool) displayLines(w io.Writer, lines []Line) (outputCount int, totalCount int) {
	usingGrep := t.hasGrep()

	t.contextualize(lines)
	t.applyLineBudgetFilter(lines)

	for _, line := range lines {
		if !line.SelectedForOutput {
			continue
		}
		if line.Error != nil {
			panic("we should not have error testLines here")
		}
		prefix := ""
		if usingGrep {
			if line.SelectedByGrep {
				prefix = "*"
			} else {
				prefix = " "
			}
		}
		fmt.Fprintf(w, "%s[%d]: %s\n", prefix, line.Number, line.Content)
		outputCount++
		if line.Number > totalCount {
			totalCount = line.Number
		}
	}
	return
}

func collect(ch <-chan Line) []Line {
	var all []Line
	for l := range ch {
		all = append(all, l)
	}
	return all
}

func (t *BashTool) filter(lines <-chan Line) []Line {
	lines = t.applyGrepExcludeFilter(lines)
	lines = t.applyGrepFilter(lines)
	lines = t.filterEmptyLines(lines)
	allLines := collect(lines)
	return allLines
}

func (t *BashTool) handleCompletedCommand(ctx context.Context) {
	exitCodeBytes, err := os.ReadFile(t.exitFile)
	if err != nil {
		if os.IsNotExist(err) {
			t.warnf("exit file %s does not exist, command may not have completed", t.exitFile)
		} else {
			t.warnf("failed to read exit file %s: %v", t.exitFile, err)
		}
		t.returnError = true
	} else {
		exitCode := strings.TrimSpace(string(exitCodeBytes))
		if exitCode != "0" {
			t.warnf("command FAILED with exit code: %s", exitCode)
			t.returnError = true
		} else {
			t.returnError = false
		}
	}

	lines := t.filter(readLines(t.outputFile))

	outputCount, totalCount := t.displayLines(&t.resultBuf, lines)

	if !t.returnError && t.resultBuf.Len() == 0 {
		if totalCount > 0 && t.Grep != "" || t.GrepExclude != "" {
			fmt.Fprintf(&t.resultBuf, "command completed successfully but no output matched, full output in %s\n", t.outputFile)
		} else {
			fmt.Fprint(&t.resultBuf, "completed successfully but produced no output")
		}
	}

	if outputCount < totalCount {
		fmt.Fprintf(&t.resultBuf, "full output available in: %s\n", t.outputFile)
	}
}

func sessionExists(ctx context.Context, sessionName string) bool {
	_, err := runTmuxCommand(ctx, "has-session", "-t", sessionName)
	if err != nil {
		if strings.Contains(err.Error(), "can't find session") {
			return false
		} else {
			panic(fmt.Sprintf("failed to check session existence: %v", err))
		}
	}
	return true
}

func (t *BashTool) checkScript() error {
	script := strings.TrimSpace(t.Command)
	if strings.HasSuffix(script, "2>&1") {
		t.warnf("stderr will always be returned, you do not need to specify 2>&1")
	}
	pipes := strings.Split(script, "|")
	for i, pipe := range pipes {
		pipes[i] = strings.TrimSpace(pipe)
	}
	if len(pipes) > 1 {
		last := pipes[len(pipes)-1]
		if strings.HasPrefix(last, "tail") {
			return fmt.Errorf("do not pipe to tail, output is automatically limited to line_budget")
		}
		if strings.HasPrefix(last, "head") {
			return fmt.Errorf("do not pipe to head, output is automatically limited to line_budget")
		}
		if strings.HasPrefix(last, "grep") {
			return fmt.Errorf("do not pipe to grep, use the grep argument instead")
		}
	}
	if strings.HasPrefix(script, "cd ") {
		return fmt.Errorf("do not use cd in the command, instead set working_directory appropriately")
	}
	if strings.HasPrefix(script, "(") || strings.HasSuffix(script, ")") {
		return fmt.Errorf("do not use subshells in the command, command is always run in a subshell")
	}
	return nil
}
