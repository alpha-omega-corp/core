package httputils

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"reflect"
	"strconv"
	"strings"

	"github.com/uptrace/bunrouter"
)

func JSON[T any](w http.ResponseWriter, res *T, err error) error {
	if err != nil {
		Error(w, err, http.StatusInternalServerError)
	}

	return bunrouter.JSON(w, res)
}

func Response[T any](w http.ResponseWriter, req func() (*T, error)) error {
	res, err := req()
	return JSON(w, res, err)
}

func GetParams[T any](w http.ResponseWriter, req bunrouter.Request) *T {
	paramsMap := req.Params().Map()
	paramsData := make(map[string]interface{}, len(paramsMap))

	for key, value := range paramsMap {
		paramsData[key] = value
	}

	if idStr, ok := paramsData["id"].(string); ok {
		if idInt, err := strconv.ParseInt(idStr, 10, 64); err == nil {
			paramsData["id"] = idInt
		}
	}

	params, err := json.Marshal(paramsData)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
	}

	data := new(T)
	if err := json.Unmarshal(params, data); err != nil {
		w.WriteHeader(http.StatusBadRequest)
	}

	return data
}

func GetBody[T any](w http.ResponseWriter, req bunrouter.Request) *T {
	data := new(T)

	if err := json.NewDecoder(req.Body).Decode(data); err != nil {
		Error(w, err, http.StatusBadRequest)
	}

	return data
}

func GetFormData[T any](w http.ResponseWriter, req bunrouter.Request) *T {
	// Ensure the form is parsed (supports both multipart and urlencoded)
	contentType := req.Header.Get("Content-Type")
	if strings.Contains(contentType, "multipart/form-data") {
		// 32 MB memory for storing file parts' metadata; values go to memory, files to temp files
		_ = req.ParseMultipartForm(32 << 20)
	} else {
		_ = req.ParseForm()
	}

	// Build expected field kinds from target type T using json tags
	expected := buildFieldInfo[T]()

	formData := make(map[string]interface{})

	// Collect scalar form values first
	if req.MultipartForm != nil && req.MultipartForm.Value != nil {
		for key, vals := range req.MultipartForm.Value {
			if len(vals) == 1 {
				if fi, ok := expected[key]; ok {
					formData[key] = parseToKind(vals[0], fi)
				} else {
					formData[key] = vals[0]
				}
			} else if len(vals) > 1 {
				// keep as []string for now
				formData[key] = vals
			}
		}
	} else if req.Form != nil {
		for key, vals := range req.Form {
			if len(vals) == 1 {
				if fi, ok := expected[key]; ok {
					formData[key] = parseToKind(vals[0], fi)
				} else {
					formData[key] = vals[0]
				}
			} else if len(vals) > 1 {
				formData[key] = vals
			}
		}
	}

	// If there are files and the target struct has matching fields, map accordingly
	if req.MultipartForm != nil && req.MultipartForm.File != nil {
		for key, files := range req.MultipartForm.File {
			if len(files) == 0 {
				continue
			}
			fi, ok := expected[key]
			if !ok {
				continue
			}
			fh := files[0]
			f, err := fh.Open()
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
				continue
			}
			dataBytes, err := io.ReadAll(f)
			_ = f.Close()
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
				continue
			}
			// If target field is []byte, set raw bytes (JSON will base64 encode).
			// If target field is string, set base64-encoded file content.
			if fi.kind == reflect.Slice && fi.elemKind == reflect.Uint8 {
				formData[key] = dataBytes
			} else if fi.kind == reflect.String {
				formData[key] = base64.StdEncoding.EncodeToString(dataBytes)
			}
		}
	}

	// Marshal to JSON then unmarshal into target type T (same pattern as GetParams)
	b, err := json.Marshal(formData)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
	}

	data := new(T)
	if err := json.Unmarshal(b, data); err != nil {
		w.WriteHeader(http.StatusBadRequest)
	}
	return data
}

// fieldInfo describes the expected type for a given json-tagged field

type fieldInfo struct {
	kind     reflect.Kind
	elemKind reflect.Kind // for slices
	bitSize  int          // for ints/floats
}

// buildFieldInfo reflects on T and maps json tag names to their kinds
func buildFieldInfo[T any]() map[string]fieldInfo {
	m := make(map[string]fieldInfo)
	t := reflect.TypeOf((*T)(nil)).Elem()
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return m
	}
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		// Skip unexported
		if f.PkgPath != "" {
			continue
		}
		jsonTag := f.Tag.Get("json")
		key := ""
		if jsonTag != "" {
			parts := strings.Split(jsonTag, ",")
			key = parts[0]
		}
		if key == "" || key == "-" {
			// Fallback to lowerCamelCase of field name if no json tag
			name := f.Name
			if len(name) > 0 {
				key = strings.ToLower(name[:1]) + name[1:]
			}
		}
		fi := fieldInfo{kind: f.Type.Kind(), elemKind: reflect.Invalid, bitSize: 0}
		if fi.kind == reflect.Slice {
			fi.elemKind = f.Type.Elem().Kind()
		}
		// determine bit sizes
		switch fi.kind {
		case reflect.Int8, reflect.Uint8:
			fi.bitSize = 8
		case reflect.Int16, reflect.Uint16:
			fi.bitSize = 16
		case reflect.Int32, reflect.Uint32:
			fi.bitSize = 32
		case reflect.Int64, reflect.Uint64:
			fi.bitSize = 64
		case reflect.Float32:
			fi.bitSize = 32
		case reflect.Float64:
			fi.bitSize = 64
		}
		m[key] = fi
	}
	return m
}

// parseToKind converts a string value into the expected kind when possible
func parseToKind(val string, fi fieldInfo) interface{} {
	switch fi.kind {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if v, err := strconv.ParseInt(val, 10, fi.bitSize); err == nil {
			return v
		}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if v, err := strconv.ParseUint(val, 10, fi.bitSize); err == nil {
			return v
		}
	case reflect.Float32, reflect.Float64:
		if v, err := strconv.ParseFloat(val, fi.bitSize); err == nil {
			return v
		}
	case reflect.Bool:
		if v, err := strconv.ParseBool(strings.ToLower(val)); err == nil {
			return v
		}
	}
	// default: keep as string
	return val
}

func Error(w http.ResponseWriter, err error, code int) {
	w.WriteHeader(code)

	log.Printf("%+v\n", err)
	_, _ = w.Write([]byte(err.Error()))
}
