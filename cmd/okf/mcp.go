package main

import (
	"bufio"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/okf-memory/okf-agent-memory/pkg/okf"
)

type jsonRPCRequest struct {
	JSONRPC string           `json:"jsonrpc"`
	ID      *json.RawMessage `json:"id,omitempty"`
	Method  string           `json:"method"`
	Params  json.RawMessage  `json:"params,omitempty"`
}

type jsonRPCResponse struct {
	JSONRPC string           `json:"jsonrpc"`
	ID      *json.RawMessage `json:"id,omitempty"`
	Result  any              `json:"result,omitempty"`
	Error   *rpcError        `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type mcpToolCallParams struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments"`
}

type mcpServer struct {
	bundleDir string
	rootDir   string
	writer    io.Writer
	mu        sync.Mutex
}

// RunMCPServer runs the Model Context Protocol stdio server for an OKF bundle.
func RunMCPServer(bundleDir string) error {
	return RunMCPServerIO(bundleDir, os.Stdin, os.Stdout)
}

const maxMCPLineLength = 4 * 1024 * 1024 // 4MB maximum JSON-RPC message size

func readBoundedLine(r *bufio.Reader, maxLen int) ([]byte, bool, error) {
	var line []byte
	for {
		chunk, isPrefix, err := r.ReadLine()
		if err != nil {
			if err == io.EOF && len(line) > 0 {
				return line, false, nil
			}
			return nil, false, err
		}
		line = append(line, chunk...)
		if len(line) > maxLen {
			for isPrefix && err == nil {
				_, isPrefix, err = r.ReadLine()
			}
			return nil, true, nil
		}
		if !isPrefix {
			break
		}
	}
	return line, false, nil
}

// RunMCPServerIO runs the MCP server on the provided reader and writer.
func RunMCPServerIO(bundleDir string, in io.Reader, out io.Writer) error {
	rootDir := os.Getenv("OKF_MCP_ROOT")
	if rootDir == "" {
		if bundleDir != "" && bundleDir != "." {
			cleanBundle := filepath.Clean(bundleDir)
			rootDir = filepath.Dir(cleanBundle)
			if rootDir == "" {
				rootDir = "."
			}
		} else {
			rootDir = "."
		}
	}

	absRoot, err := filepath.Abs(rootDir)
	if err == nil {
		if evalRoot, err := filepath.EvalSymlinks(absRoot); err == nil {
			absRoot = evalRoot
		}
		rootDir = absRoot
	}

	s := &mcpServer{
		bundleDir: bundleDir,
		rootDir:   rootDir,
		writer:    out,
	}

	reader := bufio.NewReader(in)
	for {
		line, tooLarge, err := readBoundedLine(reader, maxMCPLineLength)
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		if tooLarge {
			rawNull := json.RawMessage("null")
			s.sendError(&rawNull, -32700, "Parse error: request exceeds 4MB maximum size")
			continue
		}

		trimmed := strings.TrimSpace(string(line))
		if len(trimmed) == 0 {
			continue
		}

		var req jsonRPCRequest
		if err := json.Unmarshal(line, &req); err != nil {
			rawNull := json.RawMessage("null")
			s.sendError(&rawNull, -32700, "Parse error")
			continue
		}

		s.handleRequest(req)
	}
}

func (s *mcpServer) sendResponse(id *json.RawMessage, result any) {
	if id == nil || string(*id) == "null" {
		// Per JSON-RPC 2.0 and MCP spec, never send a response to a notification
		return
	}
	resp := jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	}
	data, _ := json.Marshal(resp)
	s.mu.Lock()
	defer s.mu.Unlock()
	_, _ = s.writer.Write(append(data, '\n'))
}

func (s *mcpServer) sendError(id *json.RawMessage, code int, message string) {
	if (id == nil || string(*id) == "null") && code != -32700 {
		// Per JSON-RPC 2.0 and MCP spec, never reply to a notification
		return
	}
	resp := jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error: &rpcError{
			Code:    code,
			Message: message,
		},
	}
	data, _ := json.Marshal(resp)
	s.mu.Lock()
	defer s.mu.Unlock()
	_, _ = s.writer.Write(append(data, '\n'))
}

func (s *mcpServer) sendToolResult(id *json.RawMessage, text string, structured any, isError bool) {
	res := map[string]any{
		"content": []map[string]string{
			{"type": "text", "text": text},
		},
	}
	if structured != nil && !isError {
		res["structuredContent"] = structured
	}
	if isError {
		res["isError"] = true
	}
	s.sendResponse(id, res)
}

func (s *mcpServer) handleRequest(req jsonRPCRequest) {
	isNotification := req.ID == nil || string(*req.ID) == "null"

	switch req.Method {
	case "initialize":
		s.sendResponse(req.ID, map[string]any{
			"protocolVersion": "2024-11-05",
			"serverInfo": map[string]string{
				"name":    "okf-agent-memory",
				"version": Version,
			},
			"capabilities": map[string]any{
				"tools": map[string]bool{
					"listChanged": false,
				},
				"resources": map[string]bool{
					"listChanged": false,
				},
				"prompts": map[string]bool{
					"listChanged": false,
				},
			},
		})

	case "notifications/initialized", "initialized":
		// Standard lifecycle notification after initialize handshake; no response.
		return

	case "notifications/cancelled", "cancelled":
		// Notification indicating client canceled a request; no response.
		return

	case "ping":
		s.sendResponse(req.ID, map[string]any{})

	case "resources/list":
		s.sendResponse(req.ID, map[string]any{
			"resources": []any{},
		})

	case "prompts/list":
		s.sendResponse(req.ID, map[string]any{
			"prompts": []any{},
		})

	case "tools/list":
		s.sendResponse(req.ID, map[string]any{
			"tools": getMCPTools(),
		})

	case "tools/call":
		s.handleToolCall(req)

	default:
		if isNotification || strings.HasPrefix(req.Method, "notifications/") {
			// Per JSON-RPC 2.0 & MCP, ignore unknown notifications silently
			return
		}
		s.sendError(req.ID, -32601, "Method not found")
	}
}

//go:embed schemas/tools.json
var embeddedToolsJSON []byte

var cachedMCPTools []map[string]any

func init() {
	if err := json.Unmarshal(embeddedToolsJSON, &cachedMCPTools); err != nil {
		panic(fmt.Sprintf("okf: corrupt embedded schemas/tools.json: %v", err))
	}
}

func getMCPTools() []map[string]any {
	return cachedMCPTools
}

func (s *mcpServer) resolveBundleDir(callParams mcpToolCallParams) (string, error) {
	target, err := getStringArg(callParams.Arguments, "bundle", 1000, false)
	if err != nil {
		return "", err
	}
	target = strings.TrimSpace(target)

	if target == "" {
		if s.bundleDir != "" && s.bundleDir != "." {
			target = s.bundleDir
		} else if info, err := os.Stat("knowledge"); err == nil && info.IsDir() {
			target = "knowledge"
		} else {
			target = "."
		}
	}

	normTarget := strings.ReplaceAll(target, "\\", "/")

	// Confinement check: if s.rootDir is configured, target must stay within s.rootDir
	if s.rootDir != "" {
		absRoot, err := filepath.EvalSymlinks(s.rootDir)
		if err != nil {
			absRoot, err = filepath.Abs(s.rootDir)
			if err != nil {
				return "", fmt.Errorf("invalid server root directory: %w", err)
			}
		} else {
			absRoot, _ = filepath.Abs(absRoot)
		}

		var absTarget string
		if okf.IsAbsPath(normTarget) {
			absTarget = normTarget
		} else {
			absTarget = filepath.Join(s.rootDir, filepath.FromSlash(normTarget))
		}

		// Walk up to find the closest ancestor that exists and evaluate its symlinks
		curr := absTarget
		var missingParts []string
		for {
			_, lstatErr := os.Lstat(curr)
			if lstatErr == nil {
				break
			}
			missingParts = append([]string{filepath.Base(curr)}, missingParts...)
			parent := filepath.Dir(curr)
			if parent == curr {
				break
			}
			curr = parent
		}

		realCurr, err := filepath.EvalSymlinks(curr)
		if err != nil {
			return "", fmt.Errorf("failed to resolve bundle path %q: %w", target, err)
		}
		realCurr, err = filepath.Abs(realCurr)
		if err != nil {
			return "", err
		}

		parts := append([]string{realCurr}, missingParts...)
		realTarget := filepath.Join(parts...)

		rel, err := filepath.Rel(absRoot, realTarget)
		normRel := strings.ReplaceAll(rel, "\\", "/")
		if err != nil || rel == ".." || normRel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || strings.HasPrefix(normRel, "../") {
			return "", fmt.Errorf("path traversal denied: bundle directory %q escapes server root %q", target, s.rootDir)
		}

		return realTarget, nil
	}

	return target, nil
}

func getStringArg(args map[string]any, key string, maxLen int, required bool) (string, error) {
	val, ok := args[key]
	if !ok || val == nil {
		if required {
			return "", fmt.Errorf("missing required argument '%s'", key)
		}
		return "", nil
	}
	strVal, ok := val.(string)
	if !ok {
		return "", fmt.Errorf("argument '%s' must be a string", key)
	}
	if len(strVal) > maxLen {
		return "", fmt.Errorf("argument '%s' exceeds maximum length of %d bytes", key, maxLen)
	}
	return strVal, nil
}

func (s *mcpServer) handleToolCall(req jsonRPCRequest) {
	var callParams mcpToolCallParams
	if err := json.Unmarshal(req.Params, &callParams); err != nil {
		s.sendError(req.ID, -32602, "Invalid params")
		return
	}

	if callParams.Arguments == nil {
		callParams.Arguments = make(map[string]any)
	}

	bundleDir, err := s.resolveBundleDir(callParams)
	if err != nil {
		s.sendToolResult(req.ID, fmt.Sprintf("Path traversal denied: %v", err), nil, true)
		return
	}

	b, err := okf.LoadBundle(bundleDir)
	if err != nil {
		s.sendToolResult(req.ID, fmt.Sprintf("Failed to load bundle from %q: %v", bundleDir, err), nil, true)
		return
	}

	switch callParams.Name {
	case "okf_search":
		query, err := getStringArg(callParams.Arguments, "query", 10000, false)
		if err != nil {
			s.sendToolResult(req.ID, fmt.Sprintf("Invalid arguments: %v", err), nil, true)
			return
		}
		forPath, err := getStringArg(callParams.Arguments, "for_path", 1000, false)
		if err != nil {
			s.sendToolResult(req.ID, fmt.Sprintf("Invalid arguments: %v", err), nil, true)
			return
		}
		limit := 10
		if l, ok := callParams.Arguments["limit"].(float64); ok {
			if l > 0 && l <= 100 {
				limit = int(l)
			} else if l > 100 {
				limit = 100
			}
		}
		var results []okf.SearchResult
		if forPath != "" {
			results = b.SearchForPath(forPath, query, limit)
		} else {
			results = b.Search(query, limit)
		}
		if results == nil {
			results = []okf.SearchResult{}
		}
		resJSON, _ := json.Marshal(results)
		envelope := map[string]any{"results": results}
		s.sendToolResult(req.ID, string(resJSON), envelope, false)

	case "okf_show":
		conceptID, err := getStringArg(callParams.Arguments, "concept_id", 1000, true)
		if err != nil {
			s.sendToolResult(req.ID, fmt.Sprintf("Invalid arguments: %v", err), nil, true)
			return
		}
		conceptID = strings.TrimSpace(conceptID)
		if err := okf.ValidateConceptID(conceptID); err != nil {
			s.sendToolResult(req.ID, fmt.Sprintf("Invalid concept_id: %v", err), nil, true)
			return
		}
		conceptID = strings.TrimSuffix(conceptID, ".md")
		c, ok := b.Concepts[conceptID]
		if !ok {
			s.sendToolResult(req.ID, fmt.Sprintf("Concept '%s' not found in %s", conceptID, bundleDir), nil, true)
			return
		}
		resJSON, _ := json.Marshal(c)
		s.sendToolResult(req.ID, string(resJSON), c, false)

	case "okf_validate":
		strict := true
		if sVal, ok := callParams.Arguments["strict"].(bool); ok {
			strict = sVal
		}
		stale := false
		if stVal, ok := callParams.Arguments["stale"].(bool); ok {
			stale = stVal
		}
		res := okf.Validate(b, okf.ValidateOptions{Strict: strict, Drift: true, Stale: stale})
		if res.Errors == nil {
			res.Errors = []string{}
		}
		if res.Warnings == nil {
			res.Warnings = []string{}
		}
		if res.GateFindings == nil {
			res.GateFindings = []string{}
		}
		if res.BrokenLinks == nil {
			res.BrokenLinks = []okf.BrokenLink{}
		}
		if res.Orphans == nil {
			res.Orphans = []string{}
		}
		resJSON, _ := json.Marshal(res)
		s.sendToolResult(req.ID, string(resJSON), res, false)

	case "okf_create":
		conceptID, err := getStringArg(callParams.Arguments, "concept_id", 1000, true)
		if err != nil {
			s.sendToolResult(req.ID, fmt.Sprintf("Invalid arguments: %v", err), nil, true)
			return
		}
		conceptID = strings.TrimSpace(conceptID)
		if err := okf.ValidateConceptID(conceptID); err != nil {
			s.sendToolResult(req.ID, fmt.Sprintf("Invalid concept_id: %v", err), nil, true)
			return
		}
		conceptType, err := getStringArg(callParams.Arguments, "type", 1000, true)
		if err != nil {
			s.sendToolResult(req.ID, fmt.Sprintf("Invalid arguments: %v", err), nil, true)
			return
		}
		conceptType = strings.TrimSpace(conceptType)
		if conceptType == "" {
			s.sendToolResult(req.ID, "Invalid type: concept type cannot be empty or whitespace", nil, true)
			return
		}
		title, err := getStringArg(callParams.Arguments, "title", 1000, true)
		if err != nil {
			s.sendToolResult(req.ID, fmt.Sprintf("Invalid arguments: %v", err), nil, true)
			return
		}
		if strings.TrimSpace(title) == "" {
			s.sendToolResult(req.ID, "Invalid title: concept title cannot be empty or whitespace", nil, true)
			return
		}
		desc, err := getStringArg(callParams.Arguments, "description", 1000, false)
		if err != nil {
			s.sendToolResult(req.ID, fmt.Sprintf("Invalid arguments: %v", err), nil, true)
			return
		}
		body, err := getStringArg(callParams.Arguments, "body", 1024*1024, false)
		if err != nil {
			s.sendToolResult(req.ID, fmt.Sprintf("Invalid arguments: %v", err), nil, true)
			return
		}

		cleanID := strings.TrimSuffix(conceptID, ".md")
		c := &okf.Concept{
			ID:          cleanID,
			Path:        cleanID + ".md",
			Type:        conceptType,
			Title:       strings.TrimSpace(title),
			Description: desc,
			Body:        body,
		}

		if err := okf.SaveConcept(bundleDir, c, true, true, true, "agent/mcp"); err != nil {
			s.sendToolResult(req.ID, fmt.Sprintf("Failed to save concept: %v", err), nil, true)
			return
		}

		msg := fmt.Sprintf("Successfully created concept %s in %s", c.Path, bundleDir)
		structured := map[string]any{
			"success":    true,
			"concept_id": c.ID,
			"path":       c.Path,
			"bundle":     bundleDir,
			"message":    msg,
		}
		s.sendToolResult(req.ID, msg, structured, false)

	case "okf_update":
		conceptID, err := getStringArg(callParams.Arguments, "concept_id", 1000, true)
		if err != nil {
			s.sendToolResult(req.ID, fmt.Sprintf("Invalid arguments: %v", err), nil, true)
			return
		}
		conceptID = strings.TrimSpace(conceptID)
		if err := okf.ValidateConceptID(conceptID); err != nil {
			s.sendToolResult(req.ID, fmt.Sprintf("Invalid concept_id: %v", err), nil, true)
			return
		}
		cleanID := strings.TrimSuffix(conceptID, ".md")
		c, ok := b.Concepts[cleanID]
		if !ok {
			s.sendToolResult(req.ID, fmt.Sprintf("Concept '%s' not found in %s", cleanID, bundleDir), nil, true)
			return
		}

		// Work on a copy to prevent in-memory concept corruption if validation or disk write fails
		updated := *c

		if _, exists := callParams.Arguments["title"]; exists {
			title, err := getStringArg(callParams.Arguments, "title", 1000, true)
			if err != nil {
				s.sendToolResult(req.ID, fmt.Sprintf("Invalid arguments: %v", err), nil, true)
				return
			}
			if strings.TrimSpace(title) == "" {
				s.sendToolResult(req.ID, "Invalid title: concept title cannot be empty or whitespace", nil, true)
				return
			}
			updated.Title = strings.TrimSpace(title)
		}
		if _, exists := callParams.Arguments["description"]; exists {
			desc, err := getStringArg(callParams.Arguments, "description", 1000, false)
			if err != nil {
				s.sendToolResult(req.ID, fmt.Sprintf("Invalid arguments: %v", err), nil, true)
				return
			}
			updated.Description = desc
		}
		if _, exists := callParams.Arguments["body"]; exists {
			body, err := getStringArg(callParams.Arguments, "body", 1024*1024, false)
			if err != nil {
				s.sendToolResult(req.ID, fmt.Sprintf("Invalid arguments: %v", err), nil, true)
				return
			}
			updated.Body = body
		}

		if err := okf.SaveConcept(bundleDir, &updated, false, true, true, "agent/mcp"); err != nil {
			s.sendToolResult(req.ID, fmt.Sprintf("Failed to update concept: %v", err), nil, true)
			return
		}

		*c = updated

		msg := fmt.Sprintf("Successfully updated concept %s in %s", c.Path, bundleDir)
		structured := map[string]any{
			"success":    true,
			"concept_id": c.ID,
			"path":       c.Path,
			"bundle":     bundleDir,
			"message":    msg,
		}
		s.sendToolResult(req.ID, msg, structured, false)

	case "okf_relate":
		srcID, err := getStringArg(callParams.Arguments, "source_id", 1000, true)
		if err != nil {
			s.sendToolResult(req.ID, fmt.Sprintf("Invalid arguments: %v", err), nil, true)
			return
		}
		tgtID, err := getStringArg(callParams.Arguments, "target_id", 1000, true)
		if err != nil {
			s.sendToolResult(req.ID, fmt.Sprintf("Invalid arguments: %v", err), nil, true)
			return
		}
		desc, err := getStringArg(callParams.Arguments, "description", 1000, false)
		if err != nil {
			s.sendToolResult(req.ID, fmt.Sprintf("Invalid arguments: %v", err), nil, true)
			return
		}

		if err := okf.RelateConcepts(bundleDir, srcID, tgtID, desc, "agent/mcp"); err != nil {
			s.sendToolResult(req.ID, fmt.Sprintf("Failed to relate concepts: %v", err), nil, true)
			return
		}

		msg := fmt.Sprintf("Successfully linked '%s' -> '%s' in %s", srcID, tgtID, bundleDir)
		structured := map[string]any{
			"success":   true,
			"source_id": srcID,
			"target_id": tgtID,
			"bundle":    bundleDir,
			"message":   msg,
		}
		s.sendToolResult(req.ID, msg, structured, false)

	default:
		s.sendToolResult(req.ID, fmt.Sprintf("Unknown tool: %s", callParams.Name), nil, true)
	}
}
