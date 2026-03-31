package claudeagent

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestToolAnnotations_ConcurrencySafe(t *testing.T) {
	reg := NewToolRegistry()

	// Tool with ConcurrencySafe annotation.
	reg.Register(ToolDefinition{
		Name:        "safe_tool",
		Description: "a safe tool",
		Annotations: &ToolAnnotations{ConcurrencySafe: true, ReadOnly: true},
	}, func(ctx context.Context, input json.RawMessage) (string, error) {
		return "ok", nil
	})

	// Tool without annotations.
	reg.Register(ToolDefinition{
		Name:        "plain_tool",
		Description: "a plain tool",
	}, func(ctx context.Context, input json.RawMessage) (string, error) {
		return "ok", nil
	})

	if !reg.IsConcurrencySafe("safe_tool") {
		t.Error("expected safe_tool to be concurrency safe")
	}
	if reg.IsConcurrencySafe("plain_tool") {
		t.Error("expected plain_tool to NOT be concurrency safe")
	}
	if reg.IsConcurrencySafe("nonexistent") {
		t.Error("expected nonexistent tool to NOT be concurrency safe")
	}
}

func TestToolAnnotations_Preserved(t *testing.T) {
	reg := NewToolRegistry()

	annotations := &ToolAnnotations{
		ReadOnly:        true,
		Destructive:     false,
		ConcurrencySafe: true,
		SearchHint:      "search the web",
	}
	reg.Register(ToolDefinition{
		Name:        "annotated",
		Description: "an annotated tool",
		Annotations: annotations,
	}, func(ctx context.Context, input json.RawMessage) (string, error) {
		return "ok", nil
	})

	got := reg.ToolAnnotations("annotated")
	if got == nil {
		t.Fatal("expected annotations, got nil")
	}
	if !got.ReadOnly {
		t.Error("expected ReadOnly")
	}
	if got.Destructive {
		t.Error("expected not Destructive")
	}
	if !got.ConcurrencySafe {
		t.Error("expected ConcurrencySafe")
	}
	if got.SearchHint != "search the web" {
		t.Errorf("expected search hint, got %q", got.SearchHint)
	}
}

func TestToolAnnotations_NilForUnannotated(t *testing.T) {
	reg := NewToolRegistry()
	reg.Register(ToolDefinition{
		Name:        "plain",
		Description: "no annotations",
	}, func(ctx context.Context, input json.RawMessage) (string, error) {
		return "ok", nil
	})

	if got := reg.ToolAnnotations("plain"); got != nil {
		t.Errorf("expected nil annotations, got %+v", got)
	}
}

func TestToolAnnotations_MergePreserved(t *testing.T) {
	reg1 := NewToolRegistry()
	reg1.Register(ToolDefinition{
		Name:        "tool_a",
		Description: "tool a",
		Annotations: &ToolAnnotations{ReadOnly: true},
	}, func(ctx context.Context, input json.RawMessage) (string, error) {
		return "a", nil
	})

	reg2 := NewToolRegistry()
	reg2.Merge(reg1)

	if !reg2.IsConcurrencySafe("tool_a") == reg1.IsConcurrencySafe("tool_a") {
		t.Error("annotations should be consistent after merge")
	}
	got := reg2.ToolAnnotations("tool_a")
	if got == nil || !got.ReadOnly {
		t.Error("expected ReadOnly annotation preserved after merge")
	}
}

func TestToolAnnotations_JSON(t *testing.T) {
	def := ToolDefinition{
		Name:        "test",
		Description: "test tool",
		Annotations: &ToolAnnotations{
			ReadOnly:    true,
			SearchHint:  "file search",
		},
	}

	data, err := json.Marshal(def)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var got ToolDefinition
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if got.Annotations == nil {
		t.Fatal("expected annotations after round-trip")
	}
	if !got.Annotations.ReadOnly {
		t.Error("expected ReadOnly after round-trip")
	}
	if got.Annotations.SearchHint != "file search" {
		t.Errorf("expected search hint, got %q", got.Annotations.SearchHint)
	}
}

func TestToolAnnotations_NilOmittedInJSON(t *testing.T) {
	def := ToolDefinition{
		Name:        "test",
		Description: "test tool",
	}

	data, err := json.Marshal(def)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	str := string(data)
	if strings.Contains(str, "annotations") {
		t.Errorf("nil annotations should be omitted from JSON, got: %s", str)
	}
}
