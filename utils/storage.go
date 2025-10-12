package utils

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"
	"reflect"
)

// uuidV4 generates a random RFC 4122 UUID v4 string without external dependencies.
func uuidV4() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	// Set version (4) and variant (RFC 4122)
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%04x%08x",
		uint32(b[0])<<24|uint32(b[1])<<16|uint32(b[2])<<8|uint32(b[3]),
		uint16(b[4])<<8|uint16(b[5]),
		uint16(b[6])<<8|uint16(b[7]),
		uint16(b[8])<<8|uint16(b[9]),
		uint16(b[10])<<8|uint16(b[11]),
		uint32(b[12])<<24|uint32(b[13])<<16|uint32(b[14])<<8|uint32(b[15]),
	), nil
}

// WriteStorage saves a base64-encoded image found in the given struct field to the storage folder
// using a UUID filename. It updates the field to the stored path and returns the path.
func WriteStorage[T any](data *T, property string) (string, error) {
	val := reflect.ValueOf(data).Elem()
	image := val.FieldByName(property)

	decoded, err := base64.StdEncoding.DecodeString(image.String())
	if err != nil {
		return "", err
	}

	id, err := uuidV4()
	if err != nil {
		return "", err
	}

	path := fmt.Sprintf("storage/%s", id)
	if err = os.WriteFile(path, decoded, 0644); err != nil {
		return "", err
	}

	image.SetString(path)

	return path, nil
}
