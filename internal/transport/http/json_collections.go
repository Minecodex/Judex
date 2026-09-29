package httptransport

import (
	"encoding/json"
	"reflect"
)

var marshalerType = reflect.TypeOf((*json.Marshaler)(nil)).Elem()

// JSON collection fields are always []/{}; optional objects remain null.
// Work on a copy so serializing a projection cannot mutate domain state.
func normalizeCollections(data any) any {
	if data == nil {
		return nil
	}
	return normalizeValue(reflect.ValueOf(data)).Interface()
}
func normalizeValue(v reflect.Value) reflect.Value {
	if !v.IsValid() {
		return v
	}
	if v.Type().Implements(marshalerType) {
		return v
	}
	switch v.Kind() {
	case reflect.Interface:
		if v.IsNil() {
			return v
		}
		out := reflect.New(v.Type()).Elem()
		out.Set(normalizeValue(v.Elem()))
		return out
	case reflect.Pointer:
		if v.IsNil() {
			return v
		}
		out := reflect.New(v.Type().Elem())
		out.Elem().Set(normalizeValue(v.Elem()))
		return out
	case reflect.Struct:
		out := reflect.New(v.Type()).Elem()
		out.Set(v)
		for i := 0; i < v.NumField(); i++ {
			if out.Field(i).CanSet() && v.Type().Field(i).PkgPath == "" {
				out.Field(i).Set(normalizeValue(v.Field(i)))
			}
		}
		return out
	case reflect.Slice:
		if v.Type().Elem().Kind() == reflect.Uint8 {
			return v
		}
		out := reflect.MakeSlice(v.Type(), v.Len(), v.Len())
		for i := 0; i < v.Len(); i++ {
			out.Index(i).Set(normalizeValue(v.Index(i)))
		}
		return out
	case reflect.Map:
		out := reflect.MakeMapWithSize(v.Type(), v.Len())
		iter := v.MapRange()
		for iter.Next() {
			out.SetMapIndex(iter.Key(), normalizeValue(iter.Value()))
		}
		return out
	}
	return v
}
