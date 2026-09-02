// Copyright (c) 2024 Tencent Inc.
// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"reflect"
	"strings"
)

// flattenMetrics expands one MetricData collection into scalar MetricPoints.
//
// Supported shapes:
//   - scalars (int/uint/float): one point whose Name is data.Type
//   - map[string]T: recursed, each key appended to the Name
//   - structs: exported fields are recursed, the json tag used as Name suffix
//   - slices / arrays / strings / bools / nil are ignored so list-like details
//     do not become log noise
func flattenMetrics(data MetricData) []MetricPoint {
	if data.Type == "" {
		return nil
	}

	var points []MetricPoint
	base := string(data.Type)

	var walk func(name string, v any)
	walk = func(name string, v any) {
		if v == nil {
			return
		}
		rv := reflect.ValueOf(v)
		for rv.Kind() == reflect.Ptr || rv.Kind() == reflect.Interface {
			if rv.IsNil() {
				return
			}
			rv = rv.Elem()
		}

		switch rv.Kind() {
		case reflect.Map:
			if rv.Type().Key().Kind() != reflect.String {
				return
			}
			iter := rv.MapRange()
			for iter.Next() {
				walk(name+"."+iter.Key().String(), iter.Value().Interface())
			}
		case reflect.Struct:
			t := rv.Type()
			for i := 0; i < rv.NumField(); i++ {
				field := t.Field(i)
				if field.PkgPath != "" { // unexported field
					continue
				}
				suffix := field.Name
				if jn := field.Tag.Get("json"); jn != "" && jn != "-" {
					suffix = strings.Split(jn, ",")[0]
				}
				walk(name+"."+suffix, rv.Field(i).Interface())
			}
		case reflect.String, reflect.Slice, reflect.Array, reflect.Bool:
			// Ignore: strings/list-like details/bools do not fit scalar logging.
		default:
			if isNumericKind(rv.Kind()) {
				points = append(points, MetricPoint{
					Name:      name,
					Value:     numericToFloat(rv),
					Timestamp: data.Timestamp,
					Tags:      data.Tags,
				})
			}
		}
	}

	walk(base, data.Value)
	return points
}

func isNumericKind(k reflect.Kind) bool {
	switch k {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64:
		return true
	default:
		return false
	}
}

func numericToFloat(rv reflect.Value) float64 {
	return rv.Convert(reflect.TypeOf(float64(0))).Float()
}
