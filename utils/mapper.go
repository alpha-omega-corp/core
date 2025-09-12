package utils

import (
	"fmt"
	"reflect"
	"strings"
	"time"
)

// Mapper provides generic struct mapping with automatic type conversions (thx claude.ai)
type Mapper struct {
	// Optional: custom field name matching function
	FieldMatcher func(srcFieldName, dstFieldName string) bool
	// Optional: skip fields that don't exist in destination
	SkipMissingFields bool
}

// NewMapper creates a new mapper with default settings
func NewMapper() *Mapper {
	return &Mapper{
		FieldMatcher:      defaultFieldMatcher,
		SkipMissingFields: true,
	}
}

// Map copies values from src to dst, handling type conversions automatically
func (m *Mapper) Map(to, from interface{}) error {
	dstVal := reflect.ValueOf(to)
	srcVal := reflect.ValueOf(from)

	// Ensure dst is a pointer to a struct
	if dstVal.Kind() != reflect.Ptr || dstVal.Elem().Kind() != reflect.Struct {
		return fmt.Errorf("destination must be a pointer to a struct")
	}

	// Handle src being either a pointer or a struct
	if srcVal.Kind() == reflect.Ptr {
		if srcVal.IsNil() {
			return fmt.Errorf("source is nil")
		}
		srcVal = srcVal.Elem()
	}

	if srcVal.Kind() != reflect.Struct {
		return fmt.Errorf("source must be a struct or pointer to struct")
	}

	dstVal = dstVal.Elem()
	dstType := dstVal.Type()
	srcType := srcVal.Type()

	// Iterate through destination fields
	for i := 0; i < dstType.NumField(); i++ {
		dstField := dstType.Field(i)
		dstFieldVal := dstVal.Field(i)

		// Skip unexported fields
		if !dstFieldVal.CanSet() {
			continue
		}

		// Skip bun.BaseModel or other embedded structs if needed
		if dstField.Anonymous && dstField.Type.Name() == "BaseModel" {
			continue
		}

		// Find matching field in source
		srcFieldVal, found := m.findMatchingField(srcVal, srcType, dstField.Name)
		if !found {
			if !m.SkipMissingFields {
				return fmt.Errorf("field %s not found in source", dstField.Name)
			}
			continue
		}

		// Set the value with type conversion if needed
		if err := m.setFieldValue(dstFieldVal, srcFieldVal); err != nil {
			return fmt.Errorf("error setting field %s: %w", dstField.Name, err)
		}
	}

	return nil
}

// findMatchingField finds a field in src that matches the destination field name
func (m *Mapper) findMatchingField(srcVal reflect.Value, srcType reflect.Type, dstFieldName string) (reflect.Value, bool) {
	for i := 0; i < srcType.NumField(); i++ {
		srcField := srcType.Field(i)
		if m.FieldMatcher(srcField.Name, dstFieldName) {
			return srcVal.Field(i), true
		}
	}
	return reflect.Value{}, false
}

// setFieldValue sets dst field value from src, handling type conversions
func (m *Mapper) setFieldValue(dst, src reflect.Value) error {
	// Handle nil/zero values
	if !src.IsValid() {
		return nil
	}

	// Handle pointer types
	srcType := src.Type()
	dstType := dst.Type()

	// Dereference source pointer if needed
	if srcType.Kind() == reflect.Ptr {
		if src.IsNil() {
			// Set zero value for destination
			dst.Set(reflect.Zero(dstType))
			return nil
		}
		src = src.Elem()
		srcType = src.Type()
	}

	// Direct assignment if types match
	if srcType == dstType {
		dst.Set(src)
		return nil
	}

	// Handle conversions
	switch {
	case isTimeType(dstType) && isInt64Type(srcType):
		// int64 to time.Time (Unix timestamp)
		timestamp := src.Int()
		dst.Set(reflect.ValueOf(time.Unix(timestamp, 0)))
		return nil

	case isInt64Type(dstType) && isTimeType(srcType):
		// time.Time to int64 (Unix timestamp)
		t := src.Interface().(time.Time)
		dst.SetInt(t.Unix())
		return nil

	case dstType.Kind() == reflect.Ptr && srcType == dstType.Elem():
		// Source is value, destination is pointer to that type
		newPtr := reflect.New(dstType.Elem())
		newPtr.Elem().Set(src)
		dst.Set(newPtr)
		return nil

	case srcType.Kind() == reflect.Ptr && dstType == srcType.Elem():
		// Source is pointer, destination is value
		if !src.IsNil() {
			dst.Set(src.Elem())
		}
		return nil

	case dstType.Kind() == srcType.Kind():
		// Same kind, try direct conversion if possible
		if src.Type().ConvertibleTo(dstType) {
			dst.Set(src.Convert(dstType))
			return nil
		}
	}

	// Try to convert if types are convertible
	if srcType.ConvertibleTo(dstType) {
		dst.Set(src.Convert(dstType))
		return nil
	}

	return fmt.Errorf("cannot convert %s to %s", srcType, dstType)
}

// Helper functions
func isTimeType(t reflect.Type) bool {
	return t == reflect.TypeOf(time.Time{}) || t == reflect.TypeOf(&time.Time{})
}

func isInt64Type(t reflect.Type) bool {
	return t.Kind() == reflect.Int64
}

// defaultFieldMatcher matches fields by exact name (case-sensitive)
func defaultFieldMatcher(srcFieldName, dstFieldName string) bool {
	return srcFieldName == dstFieldName
}

// CaseInsensitiveFieldMatcher matches fields ignoring case
func CaseInsensitiveFieldMatcher(srcFieldName, dstFieldName string) bool {
	return strings.EqualFold(srcFieldName, dstFieldName)
}

// MapSlice maps a slice of structs from src to dst
func (m *Mapper) MapSlice(dstSlice, srcSlice interface{}) error {
	srcVal := reflect.ValueOf(srcSlice)
	dstVal := reflect.ValueOf(dstSlice)

	if srcVal.Kind() != reflect.Slice {
		return fmt.Errorf("source must be a slice")
	}

	if dstVal.Kind() != reflect.Ptr || dstVal.Elem().Kind() != reflect.Slice {
		return fmt.Errorf("destination must be a pointer to a slice")
	}

	dstSliceVal := dstVal.Elem()
	dstElemType := dstSliceVal.Type().Elem()

	// Create new slice with appropriate capacity
	newSlice := reflect.MakeSlice(dstSliceVal.Type(), 0, srcVal.Len())

	for i := 0; i < srcVal.Len(); i++ {
		// Create new element of destination type
		dstElem := reflect.New(dstElemType)

		// Map the element
		if err := m.Map(dstElem.Interface(), srcVal.Index(i).Interface()); err != nil {
			return fmt.Errorf("error mapping element %d: %w", i, err)
		}

		// Append to slice
		if dstElemType.Kind() == reflect.Ptr {
			newSlice = reflect.Append(newSlice, dstElem)
		} else {
			newSlice = reflect.Append(newSlice, dstElem.Elem())
		}
	}

	dstSliceVal.Set(newSlice)
	return nil
}
