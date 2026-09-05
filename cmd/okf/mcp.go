package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/okf-memory/okf-agent-memory/pkg/okf"
)

type jsonRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type jsonRPCResponse struct {
	JSONRPC string    `json:"jsonrpc"`
	ID      any       `json:"id"`
	Result  any       `json:"result,omitempty"`
	Error   *rpcError `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type mcpToolCallParams struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments"`
}

// RunMCPServer runs the Model Context Protocol stdio server for an OKF bundle.
func RunMCPServer(bundleDir string) error {
	reader := bufio.NewReader(os.Stdin)
	for {
		line, err := reader.ReadBytes('\n')
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}

		if len(strings.TrimSpace(string(line))) == 0 {
			continue
		}

		var req jsonRPCRequest
		if err := json.Unmarshal(line, &req); err != nil {
			sendError(req.ID, -32700, "Parse error")
			continue
		}

		handleMCPRequest(bundleDir, req)
	}
}

func sendResponse(id, result any) {
	resp := jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	}
	data, _ := json.Marshal(resp)
	fmt.Printf("%s\n", string(data))
}

func sendError(id any, code int, message string) {
	resp := jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error: &rpcError{
			Code:    code,
			Message: message,
		},
	}
	data, _ := json.Marshal(resp)
	fmt.Printf("%s\n", string(data))
}

func handleMCPRequest(bundleDir string, req jsonRPCRequest) {
	switch req.Method {
	case "initialize":
		sendResponse(req.ID, map[string]any{
			"protocolVersion": "2024-11-05",
			"serverInfo": map[string]string{
				"name":    "okf-agent-memory",
				"version": Version,
			},
			"capabilities": map[string]any{
				"tools": map[string]bool{
					"listChanged": false,
				},
			},
		})

	case "tools/list":
		sendResponse(req.ID, map[string]any{
			"tools": []map[string]any{
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
						},
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
						},
						"required": []string{"source_id", "target_id"},
					},
				},
			},
		})

	case "tools/call":
		var callParams mcpToolCallParams
		if err := json.Unmarshal(req.Params, &callParams); err != nil {
			sendError(req.ID, -32602, "Invalid params")
			return
		}

		b, err := okf.LoadBundle(bundleDir)
		if err != nil {
			sendError(req.ID, -32000, fmt.Sprintf("Failed to load bundle: %v", err))
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
			sendResponse(req.ID, map[string]any{
				"content": []map[string]string{
					{"type": "text", "text": string(resJSON)},
				},
			})

		case "okf_show":
			conceptID, _ := callParams.Arguments["concept_id"].(string)
			conceptID = strings.TrimSuffix(conceptID, ".md")
			c, ok := b.Concepts[conceptID]
			if !ok {
				sendError(req.ID, -32001, fmt.Sprintf("Concept '%s' not found", conceptID))
				return
			}
			resJSON, _ := json.Marshal(c)
			sendResponse(req.ID, map[string]any{
				"content": []map[string]string{
					{"type": "text", "text": string(resJSON)},
				},
			})

		case "okf_validate":
			strict := true
			if s, ok := callParams.Arguments["strict"].(bool); ok {
				strict = s
			}
			res := okf.Validate(b, okf.ValidateOptions{Strict: strict, Drift: true})
			resJSON, _ := json.Marshal(res)
			sendResponse(req.ID, map[string]any{
				"content": []map[string]string{
					{"type": "text", "text": string(resJSON)},
				},
			})

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
				sendError(req.ID, -32002, fmt.Sprintf("Failed to save concept: %v", err))
				return
			}

			sendResponse(req.ID, map[string]any{
				"content": []map[string]string{
					{"type": "text", "text": fmt.Sprintf("Successfully created concept %s", c.Path)},
				},
			})

		case "okf_update":
			conceptID, _ := callParams.Arguments["concept_id"].(string)
			conceptID = strings.TrimSuffix(conceptID, ".md")
			c, ok := b.Concepts[conceptID]
			if !ok {
				sendError(req.ID, -32001, fmt.Sprintf("Concept '%s' not found", conceptID))
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
				sendError(req.ID, -32002, fmt.Sprintf("Failed to update concept: %v", err))
				return
			}

			sendResponse(req.ID, map[string]any{
				"content": []map[string]string{
					{"type": "text", "text": fmt.Sprintf("Successfully updated concept %s", c.Path)},
				},
			})

		case "okf_relate":
			srcID, _ := callParams.Arguments["source_id"].(string)
			tgtID, _ := callParams.Arguments["target_id"].(string)
			desc, _ := callParams.Arguments["description"].(string)

			if err := okf.RelateConcepts(bundleDir, srcID, tgtID, desc, "agent/mcp"); err != nil {
				sendError(req.ID, -32002, fmt.Sprintf("Failed to relate concepts: %v", err))
				return
			}

			sendResponse(req.ID, map[string]any{
				"content": []map[string]string{
					{"type": "text", "text": fmt.Sprintf("Successfully linked '%s' -> '%s'", srcID, tgtID)},
				},
			})

		default:
			sendError(req.ID, -32601, fmt.Sprintf("Unknown tool: %s", callParams.Name))
		}

	default:
		sendError(req.ID, -32601, "Method not found")
	}
}
