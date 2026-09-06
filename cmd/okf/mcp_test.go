package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func runMCPConversation(t *testing.T, bundleDir string, inputs []string) []jsonRPCResponse {
	t.Helper()

	var inBuf bytes.Buffer
	for _, in := range inputs {
		inBuf.WriteString(in)
		inBuf.WriteByte('\n')
	}

	var outBuf bytes.Buffer
	err := RunMCPServerIO(bundleDir, &inBuf, &outBuf)
	if err != nil && err != io.EOF {
		t.Fatalf("RunMCPServerIO returned unexpected error: %v", err)
	}

	var responses []jsonRPCResponse
	lines := strings.Split(strings.TrimSpace(outBuf.String()), "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if len(trimmed) == 0 {
			continue
		}
		var resp jsonRPCResponse
		if err := json.Unmarshal([]byte(trimmed), &resp); err != nil {
			t.Fatalf("Failed to parse response JSON %q: %v", trimmed, err)
		}
		responses = append(responses, resp)
	}

	return responses
}

func TestMCPHandshakeAndToolsList(t *testing.T) {
	// Simulate the exact lifecycle of standard MCP clients (Antigravity, Claude, Cursor)
	inputs := []string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test-client","version":"1.0"}}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`,
	}

	responses := runMCPConversation(t, "../../knowledge", inputs)

	if len(responses) != 2 {
		t.Fatalf("Expected exactly 2 responses (notification must NOT produce response), got %d: %+v", len(responses), responses)
	}

	// First response: initialize
	r1 := responses[0]
	if string(*r1.ID) != "1" {
		t.Errorf("Expected response 1 ID '1', got %s", string(*r1.ID))
	}
	r1Map, ok := r1.Result.(map[string]any)
	if !ok {
		t.Fatalf("Expected result map in response 1, got %T", r1.Result)
	}
	if r1Map["protocolVersion"] != "2024-11-05" {
		t.Errorf("Expected protocolVersion '2024-11-05', got %v", r1Map["protocolVersion"])
	}

	// Second response: tools/list
	r2 := responses[1]
	if string(*r2.ID) != "2" {
		t.Errorf("Expected response 2 ID '2', got %s", string(*r2.ID))
	}
	r2Map, ok := r2.Result.(map[string]any)
	if !ok {
		t.Fatalf("Expected result map in response 2, got %T", r2.Result)
	}
	tools, ok := r2Map["tools"].([]any)
	if !ok || len(tools) != 6 {
		t.Fatalf("Expected 6 tools in tools/list, got %v", r2Map["tools"])
	}
}

func TestMCPNotificationsAreSilent(t *testing.T) {
	// None of these notifications should produce ANY stdout line
	inputs := []string{
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","method":"initialized"}`,
		`{"jsonrpc":"2.0","method":"notifications/cancelled","params":{"requestId":42}}`,
		`{"jsonrpc":"2.0","method":"notifications/random_unknown"}`,
	}

	responses := runMCPConversation(t, "../../knowledge", inputs)
	if len(responses) != 0 {
		t.Fatalf("Expected 0 responses for notifications, got %d: %+v", len(responses), responses)
	}
}

func TestMCPPingAndOptionalMethods(t *testing.T) {
	inputs := []string{
		`{"jsonrpc":"2.0","id":"ping-1","method":"ping"}`,
		`{"jsonrpc":"2.0","id":"res-1","method":"resources/list"}`,
		`{"jsonrpc":"2.0","id":"prompt-1","method":"prompts/list"}`,
	}

	responses := runMCPConversation(t, "../../knowledge", inputs)
	if len(responses) != 3 {
		t.Fatalf("Expected 3 responses, got %d", len(responses))
	}

	// Ping response
	if string(*responses[0].ID) != `"ping-1"` {
		t.Errorf("Expected id '\"ping-1\"', got %s", string(*responses[0].ID))
	}
	if responses[0].Error != nil {
		t.Errorf("Expected no error on ping, got: %v", responses[0].Error)
	}

	// Resources response
	if string(*responses[1].ID) != `"res-1"` {
		t.Errorf("Expected id '\"res-1\"', got %s", string(*responses[1].ID))
	}
	resMap := responses[1].Result.(map[string]any)
	if _, ok := resMap["resources"]; !ok {
		t.Errorf("Expected 'resources' key in resources/list result")
	}

	// Prompts response
	if string(*responses[2].ID) != `"prompt-1"` {
		t.Errorf("Expected id '\"prompt-1\"', got %s", string(*responses[2].ID))
	}
	promptMap := responses[2].Result.(map[string]any)
	if _, ok := promptMap["prompts"]; !ok {
		t.Errorf("Expected 'prompts' key in prompts/list result")
	}
}

func TestMCPToolCalls(t *testing.T) {
	tmpDir := t.TempDir()

	// Initialize bundle in tmpDir
	inputs := []string{
		// 1. Create a concept
		`{"jsonrpc":"2.0","id":10,"method":"tools/call","params":{"name":"okf_create","arguments":{"concept_id":"decisions/test-concept","type":"Decision","title":"Test Concept","description":"A test concept.","body":"# Test Body"}}}`,
		// 2. Search
		`{"jsonrpc":"2.0","id":11,"method":"tools/call","params":{"name":"okf_search","arguments":{"query":"test"}}}`,
		// 3. Show
		`{"jsonrpc":"2.0","id":12,"method":"tools/call","params":{"name":"okf_show","arguments":{"concept_id":"decisions/test-concept"}}}`,
		// 4. Update
		`{"jsonrpc":"2.0","id":13,"method":"tools/call","params":{"name":"okf_update","arguments":{"concept_id":"decisions/test-concept","title":"Updated Title"}}}`,
		// 5. Relate
		`{"jsonrpc":"2.0","id":14,"method":"tools/call","params":{"name":"okf_create","arguments":{"concept_id":"decisions/second-concept","type":"Decision","title":"Second Concept","description":"Another test concept.","body":"# Second Body"}}}`,
		`{"jsonrpc":"2.0","id":15,"method":"tools/call","params":{"name":"okf_relate","arguments":{"source_id":"decisions/test-concept","target_id":"decisions/second-concept","description":"Related test"}}}`,
		// 6. Validate
		`{"jsonrpc":"2.0","id":16,"method":"tools/call","params":{"name":"okf_validate","arguments":{"strict":false}}}`,
		// 7. Unknown tool
		`{"jsonrpc":"2.0","id":17,"method":"tools/call","params":{"name":"non_existent_tool","arguments":{}}}`,
	}

	// Initialize basic index.md in tmpDir so LoadBundle works
	_ = os.WriteFile(filepath.Join(tmpDir, "index.md"), []byte("---\nokf_version: \"0.2\"\n---\n# Root\n"), 0o644)
	_ = os.WriteFile(filepath.Join(tmpDir, "log.md"), []byte("# Log\n"), 0o644)

	responses := runMCPConversation(t, tmpDir, inputs)
	if len(responses) != len(inputs) {
		t.Fatalf("Expected %d responses, got %d", len(inputs), len(responses))
	}

	for i, r := range responses {
		if r.Error != nil {
			t.Errorf("Step %d returned JSON-RPC error: %+v", i, r.Error)
		}
		resMap, ok := r.Result.(map[string]any)
		if !ok {
			t.Fatalf("Step %d result is not map[string]any: %T", i, r.Result)
		}
		if i == 7 { // unknown tool
			if resMap["isError"] != true {
				t.Errorf("Expected isError=true for unknown tool")
			}
		} else {
			if resMap["isError"] == true {
				t.Errorf("Step %d returned isError=true: %+v", i, resMap)
			}
		}
	}
}

func TestMCPUnknownMethodAndParseError(t *testing.T) {
	inputs := []string{
		`invalid json line`,
		`{"jsonrpc":"2.0","id":99,"method":"unknown_method"}`,
	}

	responses := runMCPConversation(t, "../../knowledge", inputs)
	if len(responses) != 2 {
		t.Fatalf("Expected 2 responses, got %d", len(responses))
	}

	// Parse error
	if responses[0].Error == nil || responses[0].Error.Code != -32700 {
		t.Errorf("Expected parse error (-32700), got: %+v", responses[0].Error)
	}

	// Method not found
	if responses[1].Error == nil || responses[1].Error.Code != -32601 {
		t.Errorf("Expected method not found (-32601), got: %+v", responses[1].Error)
	}
	if string(*responses[1].ID) != "99" {
		t.Errorf("Expected ID 99, got %s", string(*responses[1].ID))
	}
}

func TestMCPDynamicBundleResolution(t *testing.T) {
	tmpDir := t.TempDir()
	bundleA := filepath.Join(tmpDir, "bundleA")
	bundleB := filepath.Join(tmpDir, "bundleB")

	_ = os.MkdirAll(bundleA, 0o755)
	_ = os.MkdirAll(bundleB, 0o755)
	_ = os.WriteFile(filepath.Join(bundleA, "index.md"), []byte("---\nokf_version: \"0.2\"\n---\n# Bundle A\n"), 0o644)
	_ = os.WriteFile(filepath.Join(bundleA, "log.md"), []byte("# Log\n"), 0o644)
	_ = os.WriteFile(filepath.Join(bundleB, "index.md"), []byte("---\nokf_version: \"0.2\"\n---\n# Bundle B\n"), 0o644)
	_ = os.WriteFile(filepath.Join(bundleB, "log.md"), []byte("# Log\n"), 0o644)

	inputs := []string{
		// Create concept in bundle A
		`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"okf_create","arguments":{"bundle":"` + bundleA + `","concept_id":"alpha","type":"Fact","title":"Alpha","description":"Alpha in A."}}}`,
		// Create concept in bundle B
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"okf_create","arguments":{"bundle":"` + bundleB + `","concept_id":"beta","type":"Fact","title":"Beta","description":"Beta in B."}}}`,
		// Search bundle A (finds Alpha, not Beta)
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"okf_search","arguments":{"bundle":"` + bundleA + `","query":"Alpha"}}}`,
		// Search bundle B (finds Beta, not Alpha)
		`{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"okf_search","arguments":{"bundle":"` + bundleB + `","query":"Beta"}}}`,
	}

	responses := runMCPConversation(t, ".", inputs)
	if len(responses) != 4 {
		t.Fatalf("Expected 4 responses, got %d", len(responses))
	}
	for i, r := range responses {
		if r.Error != nil {
			t.Fatalf("Response %d failed: %+v", i+1, r.Error)
		}
	}
}
