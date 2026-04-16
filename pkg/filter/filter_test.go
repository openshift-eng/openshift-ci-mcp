package filter_test

import (
	"encoding/json"
	"testing"

	"github.com/jeroche/openshift-ci-mcp/pkg/filter"
)

func TestBuild_SingleArch(t *testing.T) {
	result, err := filter.Build(filter.VariantParams{Arch: "arm64"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var f filter.Filter
	if err := json.Unmarshal([]byte(result), &f); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if f.LinkOperator != "and" {
		t.Errorf("expected linkOperator 'and', got %q", f.LinkOperator)
	}
	if len(f.Items) != 1 {
		t.Fatalf("expected 1 filter item, got %d", len(f.Items))
	}
	if f.Items[0].ColumnField != "variants" {
		t.Errorf("expected columnField 'variants', got %q", f.Items[0].ColumnField)
	}
	if f.Items[0].OperatorValue != "contains" {
		t.Errorf("expected operatorValue 'contains', got %q", f.Items[0].OperatorValue)
	}
	if f.Items[0].Value != "Architecture:arm64" {
		t.Errorf("expected value 'Architecture:arm64', got %q", f.Items[0].Value)
	}
}

func TestBuild_MultipleVariants(t *testing.T) {
	result, err := filter.Build(filter.VariantParams{
		Arch:     "amd64",
		Topology: "single",
		Platform: "aws",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var f filter.Filter
	if err := json.Unmarshal([]byte(result), &f); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(f.Items) != 3 {
		t.Fatalf("expected 3 filter items, got %d", len(f.Items))
	}
}

func TestBuild_ExplicitOverridesMap(t *testing.T) {
	result, err := filter.Build(filter.VariantParams{
		Arch:     "arm64",
		Variants: map[string]string{"Architecture": "amd64"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var f filter.Filter
	if err := json.Unmarshal([]byte(result), &f); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(f.Items) != 1 {
		t.Fatalf("expected 1 filter item, got %d", len(f.Items))
	}
	if f.Items[0].Value != "Architecture:arm64" {
		t.Errorf("explicit arch should override map, got %q", f.Items[0].Value)
	}
}

func TestBuild_Empty(t *testing.T) {
	result, err := filter.Build(filter.VariantParams{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "" {
		t.Errorf("expected empty string for no variants, got %q", result)
	}
}

func TestBuild_CustomVariants(t *testing.T) {
	result, err := filter.Build(filter.VariantParams{
		Variants: map[string]string{
			"Installer":    "upi",
			"SecurityMode": "fips",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var f filter.Filter
	if err := json.Unmarshal([]byte(result), &f); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(f.Items) != 2 {
		t.Fatalf("expected 2 filter items, got %d", len(f.Items))
	}
}

func TestMergeInto_AddsFilterToExistingParams(t *testing.T) {
	params := map[string]string{"release": "4.18"}
	err := filter.MergeInto(params, filter.VariantParams{Arch: "arm64"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if params["release"] != "4.18" {
		t.Error("existing params should be preserved")
	}
	if params["filter"] == "" {
		t.Error("filter param should be set")
	}
}

func TestMergeInto_NoOpWhenEmpty(t *testing.T) {
	params := map[string]string{"release": "4.18"}
	err := filter.MergeInto(params, filter.VariantParams{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := params["filter"]; ok {
		t.Error("filter param should not be set when no variants")
	}
}
