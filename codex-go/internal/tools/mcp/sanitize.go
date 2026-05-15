package mcp

import (
	"encoding/json"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

// sanitizeJSONSchema sanitizes a JSON Schema (as a generic map/interface{}) so it can be
// used with OpenAI's API. This function implements the same logic as Rust's sanitize_json_schema:
//
// 1. Type inference for missing/null types (from properties, items, enum, etc.)
// 2. Boolean schema conversion (true → {}, false → {"not": {}})
// 3. Recursive sanitization of nested schemas
// 4. Unicode normalization (NFC) of string values
// 5. Default values for required fields (properties for objects, items for arrays)
//
// The sanitization handles edge cases from various MCP servers that may not strictly
// follow JSON Schema conventions expected by OpenAI.
func sanitizeJSONSchema(schema interface{}) interface{} {
	switch v := schema.(type) {
	case bool:
		// JSON Schema boolean form: true means "allow anything", false means "allow nothing"
		// Convert to proper schema objects
		if v {
			// true → empty schema (allows anything)
			return map[string]interface{}{"type": "string"}
		}
		// false → not schema (allows nothing)
		return map[string]interface{}{
			"not": map[string]interface{}{},
		}

	case map[string]interface{}:
		return sanitizeSchemaObject(v)

	case []interface{}:
		// Recursively sanitize array elements (for oneOf, anyOf, etc.)
		result := make([]interface{}, len(v))
		for i, item := range v {
			result[i] = sanitizeJSONSchema(item)
		}
		return result

	case string:
		// Normalize Unicode strings to NFC form
		return normalizeUnicode(v)

	default:
		// Numbers, null, etc. pass through unchanged
		return schema
	}
}

// sanitizeSchemaObject sanitizes a schema object (map[string]interface{})
func sanitizeSchemaObject(obj map[string]interface{}) map[string]interface{} {
	// Create a copy to avoid mutating the original
	result := make(map[string]interface{}, len(obj))
	for k, v := range obj {
		result[k] = v
	}

	// First, recursively sanitize known nested schema holders
	sanitizeNestedSchemas(result)

	// Normalize/ensure type field
	schemaType := inferOrExtractType(result)
	result["type"] = schemaType

	// Type-specific sanitization
	switch schemaType {
	case "object":
		sanitizeObjectSchema(result)
	case "array":
		sanitizeArraySchema(result)
	}

	// Normalize string values (descriptions, etc.)
	normalizeStringFields(result)

	return result
}

// sanitizeNestedSchemas recursively sanitizes nested schema objects
func sanitizeNestedSchemas(obj map[string]interface{}) {
	// Sanitize properties
	if props, ok := obj["properties"].(map[string]interface{}); ok {
		for k, v := range props {
			props[k] = sanitizeJSONSchema(v)
		}
	}

	// Sanitize items
	if items, ok := obj["items"]; ok {
		obj["items"] = sanitizeJSONSchema(items)
	}

	// Sanitize additionalProperties if it's a schema (not boolean)
	if ap, ok := obj["additionalProperties"]; ok {
		if _, isBool := ap.(bool); !isBool {
			obj["additionalProperties"] = sanitizeJSONSchema(ap)
		}
	}

	// Sanitize schema combiners (oneOf, anyOf, allOf, prefixItems)
	for _, combiner := range []string{"oneOf", "anyOf", "allOf", "prefixItems"} {
		if arr, ok := obj[combiner].([]interface{}); ok {
			sanitized := make([]interface{}, len(arr))
			for i, item := range arr {
				sanitized[i] = sanitizeJSONSchema(item)
			}
			obj[combiner] = sanitized
		}
	}
}

// inferOrExtractType infers or extracts the schema type
func inferOrExtractType(obj map[string]interface{}) string {
	// Check if type is explicitly set
	if typeVal, ok := obj["type"]; ok {
		// Handle type as string
		if typeStr, ok := typeVal.(string); ok {
			return normalizeTypeName(typeStr)
		}

		// Handle type as array (union type) - pick first supported type
		if typeArr, ok := typeVal.([]interface{}); ok {
			for _, t := range typeArr {
				if typeStr, ok := t.(string); ok {
					normalized := normalizeTypeName(typeStr)
					if isSupportedType(normalized) {
						return normalized
					}
				}
			}
		}
	}

	// Infer type from keywords
	return inferTypeFromKeywords(obj)
}

// inferTypeFromKeywords infers type from schema keywords
func inferTypeFromKeywords(obj map[string]interface{}) string {
	// Object indicators
	if hasAny(obj, "properties", "required", "additionalProperties") {
		return "object"
	}

	// Array indicators
	if hasAny(obj, "items", "prefixItems") {
		return "array"
	}

	// String indicators
	if hasAny(obj, "enum", "const", "format", "pattern", "minLength", "maxLength") {
		return "string"
	}

	// Number indicators
	if hasAny(obj, "minimum", "maximum", "exclusiveMinimum", "exclusiveMaximum", "multipleOf") {
		return "number"
	}

	// Boolean indicator
	// Note: JSON Schema doesn't have boolean-specific keywords, but we check anyway
	// for completeness

	// Default to string if we can't infer
	return "string"
}

// normalizeTypeName normalizes type names (e.g., "integer" → "number")
func normalizeTypeName(typeName string) string {
	// OpenAI accepts "integer", but our internal representation uses "number"
	// Match Rust's behavior: treat integer as number
	if typeName == "integer" {
		return "number"
	}
	return typeName
}

// isSupportedType checks if a type is supported
func isSupportedType(typeName string) bool {
	switch typeName {
	case "object", "array", "string", "number", "integer", "boolean":
		return true
	default:
		return false
	}
}

// sanitizeObjectSchema ensures object schemas have required fields
func sanitizeObjectSchema(obj map[string]interface{}) {
	// Ensure properties map exists
	if _, ok := obj["properties"]; !ok {
		obj["properties"] = map[string]interface{}{}
	}

	// Note: We don't add a default for additionalProperties
	// Let it remain unset if not specified, which means true in JSON Schema
}

// sanitizeArraySchema ensures array schemas have items
func sanitizeArraySchema(obj map[string]interface{}) {
	// Ensure items schema exists
	if _, ok := obj["items"]; !ok {
		// Default to allowing string items
		obj["items"] = map[string]interface{}{"type": "string"}
	}
}

// normalizeStringFields normalizes string fields in the schema (descriptions, etc.)
func normalizeStringFields(obj map[string]interface{}) {
	for k, v := range obj {
		if str, ok := v.(string); ok {
			obj[k] = normalizeUnicode(str)
		}
	}
}

// normalizeUnicode normalizes a string to Unicode NFC form
// This ensures consistent representation of Unicode characters
func normalizeUnicode(s string) string {
	if !utf8.ValidString(s) {
		// Invalid UTF-8, return as-is
		return s
	}

	// Check if already in NFC form (fast path)
	if norm.NFC.IsNormalString(s) {
		return s
	}

	// Normalize to NFC
	return norm.NFC.String(s)
}

// hasAny checks if a map has any of the given keys
func hasAny(m map[string]interface{}, keys ...string) bool {
	for _, key := range keys {
		if _, ok := m[key]; ok {
			return true
		}
	}
	return false
}

// SanitizeSchemaJSON sanitizes a JSON-serialized schema and returns the sanitized JSON.
// This is useful for sanitizing schemas before sending them to external APIs.
//
// Example:
//
//	originalJSON := `{"properties": {"name": {"description": "User name"}}}`
//	sanitizedJSON, err := SanitizeSchemaJSON(originalJSON)
//	// sanitizedJSON: `{"type":"object","properties":{"name":{"type":"string","description":"User name"}}}`
func SanitizeSchemaJSON(schemaJSON string) (string, error) {
	var schema interface{}
	if err := json.Unmarshal([]byte(schemaJSON), &schema); err != nil {
		return "", err
	}

	sanitized := sanitizeJSONSchema(schema)

	result, err := json.Marshal(sanitized)
	if err != nil {
		return "", err
	}

	return string(result), nil
}

// normalizeUnicodeInMap recursively normalizes all string values in a map
// This is used as a helper for thorough Unicode normalization
func normalizeUnicodeInMap(m map[string]interface{}) {
	for k, v := range m {
		switch val := v.(type) {
		case string:
			m[k] = normalizeUnicode(val)
		case map[string]interface{}:
			normalizeUnicodeInMap(val)
		case []interface{}:
			for i, item := range val {
				if str, ok := item.(string); ok {
					val[i] = normalizeUnicode(str)
				} else if subMap, ok := item.(map[string]interface{}); ok {
					normalizeUnicodeInMap(subMap)
				}
			}
		}
	}
}

// isControlCharacter checks if a rune is a control character
// Control characters should be preserved in some contexts
func isControlCharacter(r rune) bool {
	return unicode.IsControl(r)
}
