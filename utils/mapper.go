package utils

import (
	"fmt"
	"reflect"
	"strings"
	"time"
)

// MapperConfig holds configuration for field mapping
type MapperConfig struct {
	// FieldMappings maps source field names to target field names
	FieldMappings map[string]string
	// IgnoreFields specifies fields to skip during mapping
	IgnoreFields map[string]bool
	// CustomConverters for specific field types
	CustomConverters map[reflect.Type]func(interface{}) interface{}
}

// DefaultConfig returns a default mapper configuration
func DefaultConfig() *MapperConfig {
	return &MapperConfig{
		FieldMappings: make(map[string]string),
		IgnoreFields:  make(map[string]bool),
		CustomConverters: map[reflect.Type]func(interface{}) interface{}{
			// Map time.Time -> int64 (Unix milliseconds)
			reflect.TypeOf(time.Time{}): func(v interface{}) interface{} {
				if t, ok := v.(time.Time); ok {
					if t.IsZero() {
						return int64(0)
					}
					return t.UnixMilli()
				}
				return int64(0)
			},
			// Map *time.Time -> int64 (Unix milliseconds)
			reflect.TypeOf((*time.Time)(nil)): func(v interface{}) interface{} {
				if pt, ok := v.(*time.Time); ok {
					if pt == nil || pt.IsZero() {
						return int64(0)
					}
					return pt.UnixMilli()
				}
				return int64(0)
			},
		},
	}
}

// GenericMapper provides reflection-based mapping between structs
type GenericMapper struct {
	config *MapperConfig
}

// NewMapper creates a new generic mapper with configuration
func NewMapper(config *MapperConfig) *GenericMapper {
	if config == nil {
		config = DefaultConfig()
	}
	return &GenericMapper{config: config}
}

// MapStruct maps from source struct to target struct using reflection
func (gm *GenericMapper) MapStruct(src interface{}, target interface{}) error {
	srcVal := reflect.ValueOf(src)
	targetVal := reflect.ValueOf(target)

	// Handle pointers
	if srcVal.Kind() == reflect.Ptr {
		if srcVal.IsNil() {
			return fmt.Errorf("source is nil")
		}
		srcVal = srcVal.Elem()
	}

	if targetVal.Kind() != reflect.Ptr {
		return fmt.Errorf("target must be a pointer")
	}

	if targetVal.IsNil() {
		return fmt.Errorf("target pointer is nil")
	}

	targetVal = targetVal.Elem()

	if srcVal.Kind() != reflect.Struct || targetVal.Kind() != reflect.Struct {
		return fmt.Errorf("both source and target must be structs")
	}

	return gm.mapStructFields(srcVal, targetVal)
}

// mapStructFields performs the actual field mapping
func (gm *GenericMapper) mapStructFields(srcVal, targetVal reflect.Value) error {
	srcType := srcVal.Type()
	targetType := targetVal.Type()

	// Create a map of target fields for quick lookup
	targetFields := make(map[string]reflect.Value)
	targetFieldTypes := make(map[string]reflect.Type)

	for i := 0; i < targetType.NumField(); i++ {
		field := targetType.Field(i)
		if field.IsExported() {
			fieldName := strings.ToLower(field.Name)
			targetFields[fieldName] = targetVal.Field(i)
			targetFieldTypes[fieldName] = field.Type
		}
	}

	// Map each source field to target
	for i := 0; i < srcType.NumField(); i++ {
		srcField := srcType.Field(i)
		srcFieldValue := srcVal.Field(i)

		if !srcField.IsExported() {
			continue
		}

		fieldName := srcField.Name
		lowerFieldName := strings.ToLower(fieldName)

		// Skip ignored fields
		if gm.config.IgnoreFields[fieldName] {
			continue
		}

		// Check for field mapping
		if mappedName, exists := gm.config.FieldMappings[fieldName]; exists {
			lowerFieldName = strings.ToLower(mappedName)
		}

		// Find target field
		targetFieldValue, exists := targetFields[lowerFieldName]
		if !exists {
			continue // Skip if target field doesn't exist
		}

		if !targetFieldValue.CanSet() {
			continue
		}

		// Perform the mapping
		if err := gm.mapField(srcFieldValue, targetFieldValue, targetFieldTypes[lowerFieldName]); err != nil {
			return fmt.Errorf("failed to map field %s: %w", fieldName, err)
		}
	}

	return nil
}

// mapField maps individual field values
func (gm *GenericMapper) mapField(srcVal, targetVal reflect.Value, targetType reflect.Type) error {
	srcType := srcVal.Type()

	// Handle custom converters
	if converter, exists := gm.config.CustomConverters[srcType]; exists {
		converted := converter(srcVal.Interface())
		if converted != nil {
			convertedVal := reflect.ValueOf(converted)
			if convertedVal.Type().AssignableTo(targetType) {
				targetVal.Set(convertedVal)
				return nil
			}
			// Allow numeric conversion (e.g., int64 -> other int types)
			if convertedVal.Type().ConvertibleTo(targetType) {
				targetVal.Set(convertedVal.Convert(targetType))
				return nil
			}
		}
	}

	// Handle nil pointers
	if srcVal.Kind() == reflect.Ptr && srcVal.IsNil() {
		return nil
	}

	// Dereference pointers
	if srcVal.Kind() == reflect.Ptr {
		srcVal = srcVal.Elem()
		srcType = srcVal.Type()
	}

	switch srcVal.Kind() {
	case reflect.Slice:
		return gm.mapSlice(srcVal, targetVal, targetType)
	case reflect.Struct:
		return gm.mapStructField(srcVal, targetVal, targetType)
	default:
		return gm.mapPrimitive(srcVal, targetVal, targetType)
	}
}

// mapSlice handles slice field mapping
func (gm *GenericMapper) mapSlice(srcVal, targetVal reflect.Value, targetType reflect.Type) error {
	if srcVal.Len() == 0 {
		return nil
	}

	if targetType.Kind() != reflect.Slice {
		return fmt.Errorf("target is not a slice")
	}

	elemType := targetType.Elem()
	targetSlice := reflect.MakeSlice(targetType, srcVal.Len(), srcVal.Len())

	for i := 0; i < srcVal.Len(); i++ {
		srcElem := srcVal.Index(i)
		targetElem := targetSlice.Index(i)

		if elemType.Kind() == reflect.Ptr {
			// Create new instance for pointer elements
			newElem := reflect.New(elemType.Elem())
			if err := gm.mapField(srcElem, newElem.Elem(), elemType.Elem()); err != nil {
				return err
			}
			targetElem.Set(newElem)
		} else {
			// Direct mapping for value elements
			if err := gm.mapField(srcElem, targetElem, elemType); err != nil {
				return err
			}
		}
	}

	targetVal.Set(targetSlice)
	return nil
}

// mapStructField handles nested struct mapping
func (gm *GenericMapper) mapStructField(srcVal, targetVal reflect.Value, targetType reflect.Type) error {
	if targetType.Kind() == reflect.Ptr {
		// Create new instance for pointer target
		newTarget := reflect.New(targetType.Elem())
		if err := gm.mapStructFields(srcVal, newTarget.Elem()); err != nil {
			return err
		}
		targetVal.Set(newTarget)
	} else {
		// Direct struct mapping
		return gm.mapStructFields(srcVal, targetVal)
	}
	return nil
}

// mapPrimitive handles primitive type mapping
func (gm *GenericMapper) mapPrimitive(srcVal, targetVal reflect.Value, targetType reflect.Type) error {
	srcType := srcVal.Type()

	// Special-case: map int64 (milliseconds) -> time.Time or *time.Time
	timeType := reflect.TypeOf(time.Time{})
	ptrTimeType := reflect.TypeOf((*time.Time)(nil)).Elem() // same as timeType, kept for clarity
	if targetType == timeType || (targetType.Kind() == reflect.Ptr && targetType.Elem() == timeType) {
		// Support various integer kinds by converting to int64 first if possible
		switch srcType.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
			reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			// Convert to int64 milliseconds
			var ms int64
			if srcType.Kind() == reflect.Uint || srcType.Kind() == reflect.Uint8 || srcType.Kind() == reflect.Uint16 || srcType.Kind() == reflect.Uint32 || srcType.Kind() == reflect.Uint64 {
				ms = int64(srcVal.Convert(reflect.TypeOf(uint64(0))).Uint())
			} else {
				ms = srcVal.Convert(reflect.TypeOf(int64(0))).Int()
			}
			if ms == 0 {
				// Zero milliseconds maps to zero time
				if targetType == timeType {
					targetVal.Set(reflect.ValueOf(time.Time{}))
				} else {
					zero := time.Time{}
					ptr := reflect.New(ptrTimeType)
					ptr.Elem().Set(reflect.ValueOf(zero))
					targetVal.Set(ptr)
				}
				return nil
			}
			t := time.UnixMilli(ms)
			if targetType == timeType {
				targetVal.Set(reflect.ValueOf(t))
			} else {
				ptr := reflect.New(ptrTimeType)
				ptr.Elem().Set(reflect.ValueOf(t))
				targetVal.Set(ptr)
			}
			return nil
		}
	}

	// Direct assignment if types are assignable
	if srcType.AssignableTo(targetType) {
		targetVal.Set(srcVal)
		return nil
	}

	// Type conversion if possible
	if srcType.ConvertibleTo(targetType) {
		targetVal.Set(srcVal.Convert(targetType))
		return nil
	}

	return fmt.Errorf("cannot map %s to %s", srcType, targetType)
}

// Convenience functions for common use cases

// ModelToProto converts any model struct to proto struct
func ModelToProto(model interface{}, proto interface{}) error {
	mapper := NewMapper(DefaultConfig())
	return mapper.MapStruct(model, proto)
}

// ProtoToModel converts any proto struct to model struct
func ProtoToModel(proto interface{}, model interface{}) error {
	// If you need reverse conversion from int64 timestamps to time.Time,
	// configure a converter here accordingly (e.g., millis -> time.Time).
	mapper := NewMapper(DefaultConfig())
	return mapper.MapStruct(proto, model)
}

// MapSlice maps a slice of structs to another slice type
func MapSlice(srcSlice interface{}, targetSlicePtr interface{}) error {
	srcVal := reflect.ValueOf(srcSlice)
	targetVal := reflect.ValueOf(targetSlicePtr)

	if srcVal.Kind() != reflect.Slice {
		return fmt.Errorf("source must be a slice")
	}

	if targetVal.Kind() != reflect.Ptr || targetVal.Elem().Kind() != reflect.Slice {
		return fmt.Errorf("target must be a pointer to slice")
	}

	targetSliceVal := targetVal.Elem()
	targetSliceType := targetSliceVal.Type()
	elemType := targetSliceType.Elem()

	if srcVal.Len() == 0 {
		targetSliceVal.Set(reflect.MakeSlice(targetSliceType, 0, 0))
		return nil
	}

	resultSlice := reflect.MakeSlice(targetSliceType, srcVal.Len(), srcVal.Len())
	mapper := NewMapper(DefaultConfig())

	for i := 0; i < srcVal.Len(); i++ {
		srcElem := srcVal.Index(i)
		targetElem := resultSlice.Index(i)

		if elemType.Kind() == reflect.Ptr {
			newElem := reflect.New(elemType.Elem())
			if err := mapper.MapStruct(srcElem.Interface(), newElem.Interface()); err != nil {
				return err
			}
			targetElem.Set(newElem)
		} else {
			newElem := reflect.New(elemType)
			if err := mapper.MapStruct(srcElem.Interface(), newElem.Interface()); err != nil {
				return err
			}
			targetElem.Set(newElem.Elem())
		}
	}

	targetSliceVal.Set(resultSlice)
	return nil
}
