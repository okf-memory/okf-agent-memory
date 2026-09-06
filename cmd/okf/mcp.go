package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
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
	writer    io.Writer
	mu        sync.Mutex
}

// RunMCPServer runs the Model Context Protocol stdio server for an OKF bundle.
func RunMCPServer(bundleDir string) error {
	return RunMCPServerIO(bundleDir, os.Stdin, os.Stdout)
}

// RunMCPServerIO runs the MCP server on the provided reader and writer.
func RunMCPServerIO(bundleDir string, in io.Reader, out io.Writer) error {
	s := &mcpServer{
		bundleDir: bundleDir,
		writer:    out,
	}

	reader := bufio.NewReader(in)
	for {
		line, err := reader.ReadBytes('\n')
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}

		trimmed := strings.TrimSpace(string(line))
		if len(trimmed) == 0 {
			continue
		}

		var req jsonRPCRequest
		if err := json.Unmarshal([]byte(trimmed), &req); err != nil {
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

func (s *mcpServer) sendToolResult(id *json.RawMessage, text string, isError bool) {
	res := map[string]any{
		"content": []map[string]string{
			{"type": "text", "text": text},
		},
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

func getMCPTools() []map[string]any {
	bundleProp := map[string]any{
		"type":        "string",
		"description": "Optional path to the OKF knowledge bundle directory (defaults to 'knowledge' or project bundle).",
	}

	return []map[string]any{
		{
			"name":        "okf_search",
			"description": "Search the OKF knowledge bundle for concepts by query terms, tags, and titles using in-memory BM25 scoring.",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"query": map[string]any{
						"type":        "string",
						"description": "Search terms to find matching concepts.",
					},
					"limit": map[string]any{
						"type":        "integer",
						"description": "Maximum number of results (default 10).",
					},
					"bundle": bundleProp,
				},
				"required": []string{"query"},
			},
		},
		{
			"name":        "okf_show",
			"description": "Show the full content, frontmatter, and relationships of a specific concept.",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"concept_id": map[string]any{
						"type":        "string",
						"description": "The concept ID (e.g. 'architecture/layers').",
					},
					"bundle": bundleProp,
				},
				"required": []string{"concept_id"},
			},
		},
		{
			"name":        "okf_validate",
			"description": "Validate the entire OKF bundle for conformance, broken links, orphans, and description drift.",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"strict": map[string]any{
						"type":        "boolean",
						"description": "Treat connectivity warnings as errors.",
					},
					"bundle": bundleProp,
				},
				"required": []string{},
			},
		},
		{
			"name":        "okf_create",
			"description": "Create a new concept with frontmatter and automatic bookkeeping (updating log.md and parent index.md).",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"concept_id": map[string]any{
						"type":        "string",
						"description": "Path without .md (e.g. 'decisions/auth-flow').",
					},
					"type": map[string]any{
						"type":        "string",
						"description": "The concept type (e.g. 'Decision', 'Fact', 'Process').",
					},
					"title": map[string]any{
						"type":        "string",
						"description": "Human-readable title.",
					},
					"description": map[string]any{
						"type":        "string",
						"description": "One sentence summary of the concept.",
					},
					"body": map[string]any{
						"type":        "string",
						"description": "Markdown body content.",
					},
					"bundle": bundleProp,
				},
				"required": []string{"concept_id", "type", "title", "description"},
			},
		},
		{
			"name":        "okf_update",
			"description": "Update an existing concept's title, description, or body with automatic log.md bookkeeping.",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"concept_id": map[string]any{
						"type":        "string",
						"description": "Path without .md (e.g. 'decisions/auth-flow').",
					},
					"title": map[string]any{
						"type":        "string",
						"description": "Updated human-readable title.",
					},
					"description": map[string]any{
						"type":        "string",
						"description": "Updated one-sentence summary.",
					},
					"body": map[string]any{
						"type":        "string",
						"description": "Updated markdown body content.",
					},
					"bundle": bundleProp,
				},
				"required": []string{"concept_id"},
			},
		},
		{
			"name":        "okf_relate",
			"description": "Connect two concepts with a relative link and context description.",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"source_id": map[string]any{
						"type":        "string",
						"description": "Source concept ID.",
					},
					"target_id": map[string]any{
						"type":        "string",
						"description": "Target concept ID.",
					},
					"description": map[string]any{
						"type":        "string",
						"description": "Context description explaining the relationship.",
					},
					"bundle": bundleProp,
				},
				"required": []string{"source_id", "target_id"},
			},
		},
	}
}

func (s *mcpServer) resolveBundleDir(callParams mcpToolCallParams) string {
	if bArg, ok := callParams.Arguments["bundle"].(string); ok {
		trimmed := strings.TrimSpace(bArg)
		if trimmed != "" {
			return trimmed
		}
	}
	if s.bundleDir != "" && s.bundleDir != "." {
		return s.bundleDir
	}
	if info, err := os.Stat("knowledge"); err == nil && info.IsDir() {
		return "knowledge"
	}
	return "."
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

	bundleDir := s.resolveBundleDir(callParams)

	b, err := okf.LoadBundle(bundleDir)
	if err != nil {
		s.sendToolResult(req.ID, fmt.Sprintf("Failed to load bundle from %q: %v", bundleDir, err), true)
		return
	}

	switch callParams.Name {
	case "okf_search":
		query, _ := callParams.Arguments["query"].(string)
		limit := 10
		if l, ok := callParams.Arguments["limit"].(float64); ok && l > 0 {
			limit = int(l)
		}
		results := b.Search(query, limit)
		resJSON, _ := json.Marshal(results)
		s.sendToolResult(req.ID, string(resJSON), false)

	case "okf_show":
		conceptID, _ := callParams.Arguments["concept_id"].(string)
		conceptID = strings.TrimSuffix(conceptID, ".md")
		c, ok := b.Concepts[conceptID]
		if !ok {
			s.sendToolResult(req.ID, fmt.Sprintf("Concept '%s' not found in %s", conceptID, bundleDir), true)
			return
		}
		resJSON, _ := json.Marshal(c)
		s.sendToolResult(req.ID, string(resJSON), false)

	case "okf_validate":
		strict := true
		if sVal, ok := callParams.Arguments["strict"].(bool); ok {
			strict = sVal
		}
		res := okf.Validate(b, okf.ValidateOptions{Strict: strict, Drift: true})
		resJSON, _ := json.Marshal(res)
		s.sendToolResult(req.ID, string(resJSON), false)

	case "okf_create":
		conceptID, _ := callParams.Arguments["concept_id"].(string)
		conceptType, _ := callParams.Arguments["type"].(string)
		title, _ := callParams.Arguments["title"].(string)
		desc, _ := callParams.Arguments["description"].(string)
		body, _ := callParams.Arguments["body"].(string)

		relPath := conceptID
		if !strings.HasSuffix(relPath, ".md") {
			relPath += ".md"
		}

		c := &okf.Concept{
			ID:          strings.TrimSuffix(conceptID, ".md"),
			Path:        relPath,
			Type:        conceptType,
			Title:       title,
			Description: desc,
			Body:        body,
		}

		if err := okf.SaveConcept(bundleDir, c, true, true, true, "agent/mcp"); err != nil {
			s.sendToolResult(req.ID, fmt.Sprintf("Failed to save concept: %v", err), true)
			return
		}

		s.sendToolResult(req.ID, fmt.Sprintf("Successfully created concept %s in %s", c.Path, bundleDir), false)

	case "okf_update":
		conceptID, _ := callParams.Arguments["concept_id"].(string)
		conceptID = strings.TrimSuffix(conceptID, ".md")
		c, ok := b.Concepts[conceptID]
		if !ok {
			s.sendToolResult(req.ID, fmt.Sprintf("Concept '%s' not found in %s", conceptID, bundleDir), true)
			return
		}

		if title, ok := callParams.Arguments["title"].(string); ok && title != "" {
			c.Title = title
		}
		if desc, ok := callParams.Arguments["description"].(string); ok && desc != "" {
			c.Description = desc
		}
		if body, ok := callParams.Arguments["body"].(string); ok && body != "" {
			c.Body = body
		}

		if err := okf.SaveConcept(bundleDir, c, false, true, true, "agent/mcp"); err != nil {
			s.sendToolResult(req.ID, fmt.Sprintf("Failed to update concept: %v", err), true)
			return
		}

		s.sendToolResult(req.ID, fmt.Sprintf("Successfully updated concept %s in %s", c.Path, bundleDir), false)

	case "okf_relate":
		srcID, _ := callParams.Arguments["source_id"].(string)
		tgtID, _ := callParams.Arguments["target_id"].(string)
		desc, _ := callParams.Arguments["description"].(string)

		if err := okf.RelateConcepts(bundleDir, srcID, tgtID, desc, "agent/mcp"); err != nil {
			s.sendToolResult(req.ID, fmt.Sprintf("Failed to relate concepts: %v", err), true)
			return
		}

		s.sendToolResult(req.ID, fmt.Sprintf("Successfully linked '%s' -> '%s' in %s", srcID, tgtID, bundleDir), false)

	default:
		s.sendToolResult(req.ID, fmt.Sprintf("Unknown tool: %s", callParams.Name), true)
	}
}
