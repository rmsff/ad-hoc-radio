package sim

import (
	"reflect"
	"strings"
	"testing"
)

// TestCanonicalStructs_JSONKeysStrictlyIncrease guards the §17 key order:
// json.Marshal emits struct fields in declaration order, so the JSON key
// names of every canonical struct must strictly increase.
func TestCanonicalStructs_JSONKeysStrictlyIncrease(t *testing.T) {
	for _, typ := range []reflect.Type{
		reflect.TypeFor[canonicalEvent](),
		reflect.TypeFor[canonicalOriginKey](),
		reflect.TypeFor[canonicalCopyKey](),
	} {
		previous := ""
		for i := range typ.NumField() {
			field := typ.Field(i)
			name, _, _ := strings.Cut(field.Tag.Get("json"), ",")
			if name == "" {
				t.Errorf("%s.%s: missing JSON key name", typ.Name(), field.Name)
				continue
			}
			if name <= previous {
				t.Errorf("%s.%s: JSON key %q does not follow %q", typ.Name(), field.Name, name, previous)
			}
			previous = name
		}
	}
}

// TestCanonicalEvent_CoversEventFields fails when a field is added to Event,
// FrameDeliveryMeta, OriginKey or CopyKey without a same-named field in its
// canonical struct, so a new field cannot silently drop out of event_hash.
func TestCanonicalEvent_CoversEventFields(t *testing.T) {
	// Event fields intentionally without a same-named canonicalEvent field.
	eventExceptions := map[string]string{
		// Flattened into the top-level object; its fields are checked below.
		"Delivery": "flattened",
	}
	eventType := reflect.TypeFor[Event]()
	for name := range eventExceptions {
		if _, found := eventType.FieldByName(name); !found {
			t.Errorf("stale exception: Event has no field %s", name)
		}
	}
	cases := []struct {
		source     reflect.Type
		canonical  reflect.Type
		exceptions map[string]string
	}{
		{eventType, reflect.TypeFor[canonicalEvent](), eventExceptions},
		{reflect.TypeFor[FrameDeliveryMeta](), reflect.TypeFor[canonicalEvent](), nil},
		{reflect.TypeFor[OriginKey](), reflect.TypeFor[canonicalOriginKey](), nil},
		{reflect.TypeFor[CopyKey](), reflect.TypeFor[canonicalCopyKey](), nil},
	}
	for _, tc := range cases {
		for i := range tc.source.NumField() {
			field := tc.source.Field(i)
			if !field.IsExported() {
				continue
			}
			if _, excepted := tc.exceptions[field.Name]; excepted {
				continue
			}
			if _, found := tc.canonical.FieldByName(field.Name); !found {
				t.Errorf("%s.%s has no field in %s and is not a listed exception",
					tc.source.Name(), field.Name, tc.canonical.Name())
			}
		}
	}
}
