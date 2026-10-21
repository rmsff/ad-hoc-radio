package scenario

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/rmsff/ad-hoc-radio/internal/node"
)

type schemaDoc struct {
	AnyOf                []schemaDoc          `json:"anyOf"`
	Schema               string               `json:"$schema"`
	ID                   string               `json:"$id"`
	Title                string               `json:"title"`
	Type                 string               `json:"type"`
	AdditionalProperties *bool                `json:"additionalProperties"`
	Defs                 map[string]schemaDoc `json:"$defs"`
	Required             []string             `json:"required"`
	Properties           map[string]schemaDoc `json:"properties"`
	Items                *schemaDoc           `json:"items"`
	Ref                  string               `json:"$ref"`
	Const                interface{}          `json:"const"`
	Enum                 []interface{}        `json:"enum"`
	Minimum              *json.Number         `json:"minimum"`
	MinLength            *int                 `json:"minLength"`
	Pattern              string               `json:"pattern"`
	PatternProperties    map[string]schemaDoc `json:"patternProperties"`
	PropertyNames        *schemaDoc           `json:"propertyNames"`
}

func TestMetricsSchema_DropReasonEnumMatchesRuntime(t *testing.T) {
	schema := loadMetricsSchema(t)
	enum := schema.Defs["dropReasonCounters"].PropertyNames.Enum
	got := make([]string, 0, len(enum))
	for _, raw := range enum {
		s, ok := raw.(string)
		if !ok {
			t.Fatalf("dropReasonCounters propertyNames enum contains %T, want string", raw)
		}
		got = append(got, s)
	}
	want := make([]string, 0, len(node.AllDropReasons()))
	for _, reason := range node.AllDropReasons() {
		want = append(want, string(reason))
	}
	sort.Strings(got)
	sort.Strings(want)
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("metrics schema drop reasons = %v, want runtime reasons %v", got, want)
	}
}

func TestMetricsSchemaValidatorRejectsInvalidEnumConstAndUnknownProperty(t *testing.T) {
	schema := loadMetricsSchema(t)
	raw := mustMetricsJSONForSchemaTest(t)

	tests := []struct {
		name   string
		mutate func(map[string]interface{})
	}{
		{
			name: "invalid outcome enum",
			mutate: func(root map[string]interface{}) {
				root["outcome"] = "unknown_outcome"
			},
		},
		{
			name: "invalid hash algorithm const",
			mutate: func(root map[string]interface{}) {
				determinism := root["determinism"].(map[string]interface{})
				determinism["hash_algorithm"] = "md5"
			},
		},
		{
			name: "unknown top-level property",
			mutate: func(root map[string]interface{}) {
				root["unexpected"] = true
			},
		},
		{
			name: "unknown drop reason",
			mutate: func(root map[string]interface{}) {
				relay := root["relay"].(map[string]interface{})
				dropped := relay["dropped_by_reason"].(map[string]interface{})
				dropped["not_a_drop_reason"] = json.Number("1")
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root := decodeMetricsJSONMap(t, raw)
			tc.mutate(root)
			mutated, err := json.Marshal(root)
			if err != nil {
				t.Fatalf("marshal mutated metrics failed: %v", err)
			}
			if err := validateMetricsJSONValue(mutated, schema); err == nil {
				t.Fatal("schema validator accepted invalid metrics JSON")
			}
		})
	}
}

func TestMetricsSchemaValidatorRejectsInvalidScheduledRelayTreeEvidence(t *testing.T) {
	cfg := scheduledMetricsScenarioConfig(
		ScheduledRelayTreeDirectScenarioID,
		[]uint16{2},
		scheduledMetricsLinks([2]uint16{1, 2}),
		nil,
	)
	metrics, err := RunScheduledRelayTreeScenario(cfg)
	if err != nil {
		t.Fatalf("RunScheduledRelayTreeScenario failed: %v", err)
	}
	raw, err := metrics.MarshalReportJSON()
	if err != nil {
		t.Fatalf("MarshalReportJSON failed: %v", err)
	}
	schema := loadMetricsSchema(t)
	tests := []struct {
		name   string
		mutate func(map[string]interface{})
	}{
		{
			name: "unknown evidence property",
			mutate: func(root map[string]interface{}) {
				root["scheduled_relay_tree"].(map[string]interface{})["unexpected"] = true
			},
		},
		{
			name: "missing required schedule id",
			mutate: func(root map[string]interface{}) {
				delete(root["scheduled_relay_tree"].(map[string]interface{}), "schedule_id")
			},
		},
		{
			name: "invalid timing strategy",
			mutate: func(root map[string]interface{}) {
				root["scheduled_relay_tree"].(map[string]interface{})["timing_strategy"] = "dmr"
			},
		},
		{
			name: "invalid schedule hash",
			mutate: func(root map[string]interface{}) {
				root["scheduled_relay_tree"].(map[string]interface{})["schedule_hash"] = "ABC"
			},
		},
		{
			name: "unknown selected edge property",
			mutate: func(root map[string]interface{}) {
				edges := root["scheduled_relay_tree"].(map[string]interface{})["selected_tree_edges"].([]interface{})
				edges[0].(map[string]interface{})["weight"] = json.Number("1")
			},
		},
		{
			name: "negative latency sample count",
			mutate: func(root map[string]interface{}) {
				rows := root["scheduled_relay_tree"].(map[string]interface{})["latency_by_relay_depth"].([]interface{})
				rows[0].(map[string]interface{})["sample_count"] = json.Number("-1")
			},
		},
		{
			name: "negative blocked sequence",
			mutate: func(root map[string]interface{}) {
				root["scheduled_relay_tree"].(map[string]interface{})["blocked_from_seq"] = json.Number("-1")
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root := decodeMetricsJSONMap(t, raw)
			tc.mutate(root)
			mutated, err := json.Marshal(root)
			if err != nil {
				t.Fatalf("marshal mutated metrics failed: %v", err)
			}
			if err := validateMetricsJSONValue(mutated, schema); err == nil {
				t.Fatal("schema validator accepted invalid scheduled_relay_tree evidence")
			}
		})
	}
}

func TestMetricsSchemaValidatorRejectsCyclicRef(t *testing.T) {
	schema := schemaDoc{
		Ref: "#/$defs/loop",
		Defs: map[string]schemaDoc{
			"loop": {Ref: "#/$defs/loop"},
		},
	}
	if err := validateSchemaValue("$", schema, schema, map[string]interface{}{}); err == nil {
		t.Fatal("schema validator accepted cyclic $ref")
	}
}

func mustMetricsJSONForSchemaTest(t *testing.T) []byte {
	t.Helper()
	raw, err := runScenario(t).MarshalReportJSON()
	if err != nil {
		t.Fatalf("MarshalReportJSON failed: %v", err)
	}
	return raw
}

func decodeMetricsJSONMap(t *testing.T, raw []byte) map[string]interface{} {
	t.Helper()
	var root map[string]interface{}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&root); err != nil {
		t.Fatalf("decode metrics JSON failed: %v", err)
	}
	return root
}

func validateMetricsJSONAgainstSchema(t *testing.T, raw []byte) {
	t.Helper()
	schema := loadMetricsSchema(t)
	if err := validateMetricsJSONValue(raw, schema); err != nil {
		t.Fatalf("metrics JSON failed schema validation: %v\n%s", err, raw)
	}
}

func validateMetricsJSONValue(raw []byte, schema schemaDoc) error {
	var value interface{}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&value); err != nil {
		return fmt.Errorf("decode metrics JSON: %w", err)
	}
	if err := validateSchemaValue("$", schema, schema, value); err != nil {
		return err
	}
	var trailing interface{}
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return fmt.Errorf("trailing JSON token after metrics object")
		}
		return fmt.Errorf("decode trailing JSON token: %w", err)
	}
	return nil
}

func loadMetricsSchema(t *testing.T) schemaDoc {
	t.Helper()
	path := filepath.Join("..", "..", "docs", "protocol", "metrics-schema.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read metrics schema %s failed: %v", path, err)
	}
	assertOnlySupportedSchemaKeywords(t, raw)
	var schema schemaDoc
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&schema); err != nil {
		t.Fatalf("decode metrics schema failed: %v", err)
	}
	return schema
}

func assertOnlySupportedSchemaKeywords(t *testing.T, raw []byte) {
	t.Helper()
	var schema interface{}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&schema); err != nil {
		t.Fatalf("decode metrics schema for keyword audit failed: %v", err)
	}
	supported := map[string]bool{
		"anyOf":                true,
		"$schema":              true,
		"$id":                  true,
		"title":                true,
		"type":                 true,
		"additionalProperties": true,
		"$defs":                true,
		"required":             true,
		"properties":           true,
		"items":                true,
		"$ref":                 true,
		"const":                true,
		"enum":                 true,
		"minimum":              true,
		"minLength":            true,
		"pattern":              true,
		"patternProperties":    true,
		"propertyNames":        true,
		"default":              true,
	}
	assertSchemaKeywordsSupported(t, "$", schema, supported)
}

func assertSchemaKeywordsSupported(t *testing.T, path string, value interface{}, supported map[string]bool) {
	t.Helper()
	if array, ok := value.([]interface{}); ok {
		for i, child := range array {
			assertSchemaKeywordsSupported(t, fmt.Sprintf("%s[%d]", path, i), child, supported)
		}
		return
	}
	obj, ok := value.(map[string]interface{})
	if !ok {
		return
	}
	for key, child := range obj {
		if !supported[key] && isSchemaKeywordContext(obj) {
			t.Fatalf("metrics schema uses unsupported keyword %q at %s", key, path)
		}
		nextPath := path + "." + key
		if key == "properties" || key == "$defs" || key == "patternProperties" {
			childObj, ok := child.(map[string]interface{})
			if !ok {
				t.Fatalf("schema keyword %s at %s is %T, want object", key, path, child)
			}
			for name, nested := range childObj {
				assertSchemaKeywordsSupported(t, nextPath+"."+name, nested, supported)
			}
			continue
		}
		assertSchemaKeywordsSupported(t, nextPath, child, supported)
	}
}

func isSchemaKeywordContext(obj map[string]interface{}) bool {
	for _, key := range []string{"type", "$ref", "properties", "items", "required", "enum", "const", "patternProperties", "anyOf"} {
		if _, ok := obj[key]; ok {
			return true
		}
	}
	return false
}

func validateSchemaValue(path string, schema, root schemaDoc, value interface{}) error {
	return validateSchemaValueSeen(path, schema, root, value, make(map[string]bool))
}

func validateSchemaValueSeen(path string, schema, root schemaDoc, value interface{}, seenRefs map[string]bool) error {
	if schema.Ref != "" {
		if seenRefs[schema.Ref] {
			return fmt.Errorf("%s: cyclic $ref %q", path, schema.Ref)
		}
		resolved, err := resolveSchemaRef(schema.Ref, root)
		if err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		nextSeen := copyRefSet(seenRefs)
		nextSeen[schema.Ref] = true
		return validateSchemaValueSeen(path, resolved, root, value, nextSeen)
	}
	if err := validateConst(path, schema, value); err != nil {
		return err
	}
	if err := validateEnum(path, schema, value); err != nil {
		return err
	}
	if len(schema.AnyOf) > 0 {
		matched := false
		for _, branch := range schema.AnyOf {
			if validateSchemaValueSeen(path, branch, root, value, copyRefSet(seenRefs)) == nil {
				matched = true
				break
			}
		}
		if !matched {
			return fmt.Errorf("%s: no anyOf alternative matched", path)
		}
	}
	switch schema.Type {
	case "null":
		if value != nil {
			return fmt.Errorf("%s: expected null", path)
		}
		return nil
	case "boolean":
		if _, ok := value.(bool); !ok {
			return fmt.Errorf("%s: expected boolean", path)
		}
		return nil
	case "object":
		return validateObject(path, schema, root, value, seenRefs)
	case "array":
		return validateArray(path, schema, root, value, seenRefs)
	case "integer":
		return validateInteger(path, schema, value)
	case "number":
		return validateNumber(path, schema, value)
	case "string":
		return validateString(path, schema, value)
	case "":
		return nil
	default:
		return fmt.Errorf("%s: unsupported schema type %q in test validator", path, schema.Type)
	}
}

func copyRefSet(in map[string]bool) map[string]bool {
	out := make(map[string]bool, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func resolveSchemaRef(ref string, root schemaDoc) (schemaDoc, error) {
	const prefix = "#/$defs/"
	if !strings.HasPrefix(ref, prefix) {
		return schemaDoc{}, fmt.Errorf("unsupported $ref %q", ref)
	}
	name := strings.TrimPrefix(ref, prefix)
	schema, ok := root.Defs[name]
	if !ok {
		return schemaDoc{}, fmt.Errorf("missing $defs entry %q", name)
	}
	return schema, nil
}

func validateObject(path string, schema, root schemaDoc, value interface{}, seenRefs map[string]bool) error {
	obj, ok := value.(map[string]interface{})
	if !ok {
		return fmt.Errorf("%s: got %T, want object", path, value)
	}
	for _, name := range schema.Required {
		if _, ok := obj[name]; !ok {
			return fmt.Errorf("%s: missing required property %q", path, name)
		}
	}
	if schema.PropertyNames != nil {
		for name := range obj {
			if err := validateSchemaValueSeen(path+" property name", *schema.PropertyNames, root, name, seenRefs); err != nil {
				return err
			}
		}
	}
	for name, child := range obj {
		childSchema, ok := schema.Properties[name]
		if !ok {
			var matched bool
			for pattern, patternSchema := range schema.PatternProperties {
				re, err := regexp.Compile(pattern)
				if err != nil {
					return fmt.Errorf("%s: invalid schema pattern %q: %w", path, pattern, err)
				}
				if re.MatchString(name) {
					matched = true
					childSchema = patternSchema
					break
				}
			}
			if !matched {
				if schema.AdditionalProperties != nil && !*schema.AdditionalProperties {
					return fmt.Errorf("%s: additional property %q not allowed", path, name)
				}
				continue
			}
		}
		if err := validateSchemaValueSeen(path+"."+name, childSchema, root, child, seenRefs); err != nil {
			return err
		}
	}
	return nil
}

func validateArray(path string, schema, root schemaDoc, value interface{}, seenRefs map[string]bool) error {
	arr, ok := value.([]interface{})
	if !ok {
		return fmt.Errorf("%s: got %T, want array", path, value)
	}
	if schema.Items == nil {
		return nil
	}
	for i, item := range arr {
		if err := validateSchemaValueSeen(fmt.Sprintf("%s[%d]", path, i), *schema.Items, root, item, seenRefs); err != nil {
			return err
		}
	}
	return nil
}

func validateInteger(path string, schema schemaDoc, value interface{}) error {
	num, ok := value.(json.Number)
	if !ok {
		return fmt.Errorf("%s: got %T, want integer", path, value)
	}
	if strings.ContainsAny(num.String(), ".eE") {
		return fmt.Errorf("%s: got non-integer number %q", path, num.String())
	}
	if _, ok := new(big.Int).SetString(num.String(), 10); !ok {
		return fmt.Errorf("%s: invalid integer %q", path, num.String())
	}
	return validateMinimum(path, schema, num)
}

func validateNumber(path string, schema schemaDoc, value interface{}) error {
	num, ok := value.(json.Number)
	if !ok {
		return fmt.Errorf("%s: got %T, want number", path, value)
	}
	if _, err := num.Float64(); err != nil {
		return fmt.Errorf("%s: invalid number %q: %w", path, num.String(), err)
	}
	return validateMinimum(path, schema, num)
}

func validateString(path string, schema schemaDoc, value interface{}) error {
	s, ok := value.(string)
	if !ok {
		return fmt.Errorf("%s: got %T, want string", path, value)
	}
	if schema.MinLength != nil && len(s) < *schema.MinLength {
		return fmt.Errorf("%s: string length %d < minLength %d", path, len(s), *schema.MinLength)
	}
	if schema.Pattern != "" {
		matched, err := regexp.MatchString(schema.Pattern, s)
		if err != nil {
			return fmt.Errorf("%s: invalid schema pattern %q: %w", path, schema.Pattern, err)
		}
		if !matched {
			return fmt.Errorf("%s: string %q does not match pattern %q", path, s, schema.Pattern)
		}
	}
	return nil
}

func validateMinimum(path string, schema schemaDoc, value json.Number) error {
	if schema.Minimum == nil {
		return nil
	}
	got, ok := new(big.Rat).SetString(value.String())
	if !ok {
		return fmt.Errorf("%s: invalid number %q", path, value.String())
	}
	min, ok := new(big.Rat).SetString(schema.Minimum.String())
	if !ok {
		return fmt.Errorf("%s: invalid schema minimum %q", path, schema.Minimum.String())
	}
	if got.Cmp(min) < 0 {
		return fmt.Errorf("%s: value %s < minimum %s", path, got.String(), min.String())
	}
	return nil
}

func validateConst(path string, schema schemaDoc, value interface{}) error {
	if schema.Const == nil {
		return nil
	}
	if !schemaValueEqual(value, schema.Const) {
		return fmt.Errorf("%s: value %v does not match const %v", path, value, schema.Const)
	}
	return nil
}

func validateEnum(path string, schema schemaDoc, value interface{}) error {
	if len(schema.Enum) == 0 {
		return nil
	}
	for _, candidate := range schema.Enum {
		if schemaValueEqual(value, candidate) {
			return nil
		}
	}
	return fmt.Errorf("%s: value %v not in enum %v", path, value, schema.Enum)
}

func schemaValueEqual(left, right interface{}) bool {
	leftNum, leftOK := left.(json.Number)
	rightNum, rightOK := right.(json.Number)
	if leftOK && rightOK {
		return leftNum.String() == rightNum.String()
	}
	return fmt.Sprint(left) == fmt.Sprint(right)
}
