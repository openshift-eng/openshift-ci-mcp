package filter

import (
	"encoding/json"
	"fmt"
	"sort"
)

type Item struct {
	ColumnField   string `json:"columnField"`
	OperatorValue string `json:"operatorValue"`
	Value         string `json:"value"`
}

type Filter struct {
	Items        []Item `json:"items"`
	LinkOperator string `json:"linkOperator"`
}

type VariantParams struct {
	Arch     string
	Topology string
	Platform string
	Network  string
	Variants map[string]string
}

func Build(p VariantParams) (string, error) {
	merged := make(map[string]string)
	for k, v := range p.Variants {
		merged[k] = v
	}
	if p.Arch != "" {
		merged["Architecture"] = p.Arch
	}
	if p.Topology != "" {
		merged["Topology"] = p.Topology
	}
	if p.Platform != "" {
		merged["Platform"] = p.Platform
	}
	if p.Network != "" {
		merged["Network"] = p.Network
	}

	if len(merged) == 0 {
		return "", nil
	}

	keys := make([]string, 0, len(merged))
	for k := range merged {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	f := Filter{LinkOperator: "and"}
	for _, k := range keys {
		f.Items = append(f.Items, Item{
			ColumnField:   "variants",
			OperatorValue: "contains",
			Value:         fmt.Sprintf("%s:%s", k, merged[k]),
		})
	}

	b, err := json.Marshal(f)
	if err != nil {
		return "", fmt.Errorf("marshaling filter: %w", err)
	}
	return string(b), nil
}

func MergeInto(params map[string]string, vp VariantParams) error {
	filterJSON, err := Build(vp)
	if err != nil {
		return err
	}
	if filterJSON != "" {
		params["filter"] = filterJSON
	}
	return nil
}

func MergeItemInto(params map[string]string, item Item) {
	existing := params["filter"]
	if existing == "" {
		f := Filter{
			Items:        []Item{item},
			LinkOperator: "and",
		}
		b, _ := json.Marshal(f)
		params["filter"] = string(b)
		return
	}

	var f Filter
	if err := json.Unmarshal([]byte(existing), &f); err != nil {
		f = Filter{LinkOperator: "and"}
	}
	f.Items = append(f.Items, item)
	b, _ := json.Marshal(f)
	params["filter"] = string(b)
}
