package main

import (
	"reflect"
	"testing"
)

func TestSplitOptionalPath(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		fallback string
		wantPath string
		wantArgs []string
	}{
		{
			name:     "flags only",
			args:     []string{"--limit", "3", "--json"},
			fallback: "knowledge",
			wantPath: "knowledge",
			wantArgs: []string{"--limit", "3", "--json"},
		},
		{
			name:     "explicit path before flags",
			args:     []string{"custom-bundle", "--limit", "3"},
			fallback: "knowledge",
			wantPath: "custom-bundle",
			wantArgs: []string{"--limit", "3"},
		},
		{
			name:     "valued create flags",
			args:     []string{"--type", "Fact", "--title", "Demo", "--desc", "Text"},
			fallback: "knowledge",
			wantPath: "knowledge",
			wantArgs: []string{"--type", "Fact", "--title", "Demo", "--desc", "Text"},
		},
		{
			name:     "valued update flags",
			args:     []string{"--desc", "Updated text", "--actor", "agent/test"},
			fallback: "knowledge",
			wantPath: "knowledge",
			wantArgs: []string{"--desc", "Updated text", "--actor", "agent/test"},
		},
		{
			name:     "valued relate flags",
			args:     []string{"--desc", "Relationship context", "--actor", "agent/test"},
			fallback: "knowledge",
			wantPath: "knowledge",
			wantArgs: []string{"--desc", "Relationship context", "--actor", "agent/test"},
		},
		{
			name:     "valued bootstrap flags",
			args:     []string{"--name", "Demo", "--no-skill"},
			fallback: ".",
			wantPath: ".",
			wantArgs: []string{"--name", "Demo", "--no-skill"},
		},
		{
			name:     "no arguments",
			fallback: ".",
			wantPath: ".",
			wantArgs: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotPath, gotArgs := splitOptionalPath(tt.args, tt.fallback)
			if gotPath != tt.wantPath {
				t.Fatalf("splitOptionalPath() path = %q, want %q", gotPath, tt.wantPath)
			}
			if !reflect.DeepEqual(gotArgs, tt.wantArgs) {
				t.Fatalf("splitOptionalPath() args = %#v, want %#v", gotArgs, tt.wantArgs)
			}
		})
	}
}
