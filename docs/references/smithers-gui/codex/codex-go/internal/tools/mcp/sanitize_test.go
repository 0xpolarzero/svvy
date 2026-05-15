package mcp

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSanitizeJSONSchema_BooleanSchemas tests boolean schema conversion
func TestSanitizeJSONSchema_BooleanSchemas(t *testing.T) {
	t.Run("true becomes string schema", func(t *testing.T) {
		result := sanitizeJSONSchema(true)
		expected := map[string]interface{}{"type": "string"}
		assert.Equal(t, expected, result)
	})

	t.Run("false becomes not schema", func(t *testing.T) {
		result := sanitizeJSONSchema(false)
		expected := map[string]interface{}{
			"not": map[string]interface{}{},
		}
		assert.Equal(t, expected, result)
	})

	t.Run("boolean in nested schema", func(t *testing.T) {
		schema := map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"allowAny": true,
				"allowNone": false,
			},
		}

		result := sanitizeJSONSchema(schema).(map[string]interface{})
		props := result["properties"].(map[string]interface{})

		assert.Equal(t, map[string]interface{}{"type": "string"}, props["allowAny"])
		assert.Equal(t, map[string]interface{}{"not": map[string]interface{}{}}, props["allowNone"])
	})
}

// TestSanitizeJSONSchema_TypeInference tests type inference from keywords
func TestSanitizeJSONSchema_TypeInference(t *testing.T) {
	t.Run("infer object from properties", func(t *testing.T) {
		schema := map[string]interface{}{
			"properties": map[string]interface{}{
				"name": map[string]interface{}{"type": "string"},
			},
		}

		result := sanitizeJSONSchema(schema).(map[string]interface{})
		assert.Equal(t, "object", result["type"])
	})

	t.Run("infer object from required", func(t *testing.T) {
		schema := map[string]interface{}{
			"required": []interface{}{"name"},
		}

		result := sanitizeJSONSchema(schema).(map[string]interface{})
		assert.Equal(t, "object", result["type"])
	})

	t.Run("infer object from additionalProperties", func(t *testing.T) {
		schema := map[string]interface{}{
			"additionalProperties": false,
		}

		result := sanitizeJSONSchema(schema).(map[string]interface{})
		assert.Equal(t, "object", result["type"])
	})

	t.Run("infer array from items", func(t *testing.T) {
		schema := map[string]interface{}{
			"items": map[string]interface{}{"type": "string"},
		}

		result := sanitizeJSONSchema(schema).(map[string]interface{})
		assert.Equal(t, "array", result["type"])
	})

	t.Run("infer array from prefixItems", func(t *testing.T) {
		schema := map[string]interface{}{
			"prefixItems": []interface{}{
				map[string]interface{}{"type": "string"},
			},
		}

		result := sanitizeJSONSchema(schema).(map[string]interface{})
		assert.Equal(t, "array", result["type"])
	})

	t.Run("infer string from enum", func(t *testing.T) {
		schema := map[string]interface{}{
			"enum": []interface{}{"red", "green", "blue"},
		}

		result := sanitizeJSONSchema(schema).(map[string]interface{})
		assert.Equal(t, "string", result["type"])
	})

	t.Run("infer string from format", func(t *testing.T) {
		schema := map[string]interface{}{
			"format": "email",
		}

		result := sanitizeJSONSchema(schema).(map[string]interface{})
		assert.Equal(t, "string", result["type"])
	})

	t.Run("infer number from minimum", func(t *testing.T) {
		schema := map[string]interface{}{
			"minimum": 0,
		}

		result := sanitizeJSONSchema(schema).(map[string]interface{})
		assert.Equal(t, "number", result["type"])
	})

	t.Run("infer number from maximum", func(t *testing.T) {
		schema := map[string]interface{}{
			"maximum": 100,
		}

		result := sanitizeJSONSchema(schema).(map[string]interface{})
		assert.Equal(t, "number", result["type"])
	})

	t.Run("default to string when no keywords", func(t *testing.T) {
		schema := map[string]interface{}{
			"description": "some field",
		}

		result := sanitizeJSONSchema(schema).(map[string]interface{})
		assert.Equal(t, "string", result["type"])
	})
}

// TestSanitizeJSONSchema_TypeNormalization tests type name normalization
func TestSanitizeJSONSchema_TypeNormalization(t *testing.T) {
	t.Run("integer normalized to number", func(t *testing.T) {
		schema := map[string]interface{}{
			"type": "integer",
		}

		result := sanitizeJSONSchema(schema).(map[string]interface{})
		assert.Equal(t, "number", result["type"])
	})

	t.Run("integer in nested property", func(t *testing.T) {
		schema := map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"count": map[string]interface{}{
					"type": "integer",
				},
			},
		}

		result := sanitizeJSONSchema(schema).(map[string]interface{})
		props := result["properties"].(map[string]interface{})
		count := props["count"].(map[string]interface{})
		assert.Equal(t, "number", count["type"])
	})

	t.Run("union type picks first supported", func(t *testing.T) {
		schema := map[string]interface{}{
			"type": []interface{}{"null", "string", "number"},
		}

		result := sanitizeJSONSchema(schema).(map[string]interface{})
		// Should pick "string" as it's the first supported type
		assert.Equal(t, "string", result["type"])
	})

	t.Run("union type with integer", func(t *testing.T) {
		schema := map[string]interface{}{
			"type": []interface{}{"integer", "string"},
		}

		result := sanitizeJSONSchema(schema).(map[string]interface{})
		// Should normalize integer to number
		assert.Equal(t, "number", result["type"])
	})
}

// TestSanitizeJSONSchema_DefaultValues tests default value insertion
func TestSanitizeJSONSchema_DefaultValues(t *testing.T) {
	t.Run("object gets empty properties", func(t *testing.T) {
		schema := map[string]interface{}{
			"type": "object",
		}

		result := sanitizeJSONSchema(schema).(map[string]interface{})
		assert.NotNil(t, result["properties"])
		props := result["properties"].(map[string]interface{})
		assert.Empty(t, props)
	})

	t.Run("array gets default string items", func(t *testing.T) {
		schema := map[string]interface{}{
			"type": "array",
		}

		result := sanitizeJSONSchema(schema).(map[string]interface{})
		assert.NotNil(t, result["items"])
		items := result["items"].(map[string]interface{})
		assert.Equal(t, "string", items["type"])
	})

	t.Run("object with existing properties unchanged", func(t *testing.T) {
		schema := map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"name": map[string]interface{}{"type": "string"},
			},
		}

		result := sanitizeJSONSchema(schema).(map[string]interface{})
		props := result["properties"].(map[string]interface{})
		assert.Contains(t, props, "name")
	})

	t.Run("array with existing items unchanged", func(t *testing.T) {
		schema := map[string]interface{}{
			"type": "array",
			"items": map[string]interface{}{"type": "number"},
		}

		result := sanitizeJSONSchema(schema).(map[string]interface{})
		items := result["items"].(map[string]interface{})
		assert.Equal(t, "number", items["type"])
	})
}

// TestSanitizeJSONSchema_RecursiveSanitization tests recursive processing
func TestSanitizeJSONSchema_RecursiveSanitization(t *testing.T) {
	t.Run("nested object properties", func(t *testing.T) {
		schema := map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"address": map[string]interface{}{
					"properties": map[string]interface{}{
						"street": map[string]interface{}{
							"description": "Street name",
						},
					},
				},
			},
		}

		result := sanitizeJSONSchema(schema).(map[string]interface{})
		props := result["properties"].(map[string]interface{})
		address := props["address"].(map[string]interface{})

		// Address should be inferred as object
		assert.Equal(t, "object", address["type"])

		// Street should be inferred as string
		addrProps := address["properties"].(map[string]interface{})
		street := addrProps["street"].(map[string]interface{})
		assert.Equal(t, "string", street["type"])
	})

	t.Run("array with nested items", func(t *testing.T) {
		schema := map[string]interface{}{
			"type": "array",
			"items": map[string]interface{}{
				"properties": map[string]interface{}{
					"id": map[string]interface{}{},
				},
			},
		}

		result := sanitizeJSONSchema(schema).(map[string]interface{})
		items := result["items"].(map[string]interface{})

		// Items should be inferred as object
		assert.Equal(t, "object", items["type"])

		// ID should be inferred as string
		itemProps := items["properties"].(map[string]interface{})
		id := itemProps["id"].(map[string]interface{})
		assert.Equal(t, "string", id["type"])
	})

	t.Run("additionalProperties schema", func(t *testing.T) {
		schema := map[string]interface{}{
			"type": "object",
			"additionalProperties": map[string]interface{}{
				"properties": map[string]interface{}{
					"value": map[string]interface{}{},
				},
			},
		}

		result := sanitizeJSONSchema(schema).(map[string]interface{})
		ap := result["additionalProperties"].(map[string]interface{})

		// Should be sanitized as object
		assert.Equal(t, "object", ap["type"])

		apProps := ap["properties"].(map[string]interface{})
		value := apProps["value"].(map[string]interface{})
		assert.Equal(t, "string", value["type"])
	})

	t.Run("additionalProperties boolean preserved", func(t *testing.T) {
		schema := map[string]interface{}{
			"type": "object",
			"additionalProperties": false,
		}

		result := sanitizeJSONSchema(schema).(map[string]interface{})
		assert.Equal(t, false, result["additionalProperties"])
	})
}

// TestSanitizeJSONSchema_SchemaCombinators tests oneOf, anyOf, allOf
func TestSanitizeJSONSchema_SchemaCombinators(t *testing.T) {
	t.Run("oneOf sanitized recursively", func(t *testing.T) {
		schema := map[string]interface{}{
			"oneOf": []interface{}{
				map[string]interface{}{
					"properties": map[string]interface{}{
						"name": map[string]interface{}{},
					},
				},
				map[string]interface{}{
					"items": map[string]interface{}{},
				},
			},
		}

		result := sanitizeJSONSchema(schema).(map[string]interface{})
		oneOf := result["oneOf"].([]interface{})

		// First option should be object
		first := oneOf[0].(map[string]interface{})
		assert.Equal(t, "object", first["type"])

		// Second option should be array
		second := oneOf[1].(map[string]interface{})
		assert.Equal(t, "array", second["type"])
	})

	t.Run("anyOf with boolean schemas", func(t *testing.T) {
		schema := map[string]interface{}{
			"anyOf": []interface{}{
				true,
				map[string]interface{}{"type": "string"},
			},
		}

		result := sanitizeJSONSchema(schema).(map[string]interface{})
		anyOf := result["anyOf"].([]interface{})

		// Boolean true should become string schema
		first := anyOf[0].(map[string]interface{})
		assert.Equal(t, "string", first["type"])

		second := anyOf[1].(map[string]interface{})
		assert.Equal(t, "string", second["type"])
	})

	t.Run("allOf sanitized recursively", func(t *testing.T) {
		schema := map[string]interface{}{
			"allOf": []interface{}{
				map[string]interface{}{
					"type": "object",
				},
				map[string]interface{}{
					"properties": map[string]interface{}{
						"id": map[string]interface{}{
							"type": "integer",
						},
					},
				},
			},
		}

		result := sanitizeJSONSchema(schema).(map[string]interface{})
		allOf := result["allOf"].([]interface{})

		// Check that nested schemas are sanitized
		second := allOf[1].(map[string]interface{})
		props := second["properties"].(map[string]interface{})
		id := props["id"].(map[string]interface{})
		assert.Equal(t, "number", id["type"]) // integer → number
	})

	t.Run("prefixItems sanitized", func(t *testing.T) {
		schema := map[string]interface{}{
			"type": "array",
			"prefixItems": []interface{}{
				map[string]interface{}{
					"type": "integer",
				},
				true,
			},
		}

		result := sanitizeJSONSchema(schema).(map[string]interface{})
		prefixItems := result["prefixItems"].([]interface{})

		// First should normalize integer
		first := prefixItems[0].(map[string]interface{})
		assert.Equal(t, "number", first["type"])

		// Second should convert boolean
		second := prefixItems[1].(map[string]interface{})
		assert.Equal(t, "string", second["type"])
	})
}

// TestSanitizeJSONSchema_UnicodeNormalization tests Unicode NFC normalization
func TestSanitizeJSONSchema_UnicodeNormalization(t *testing.T) {
	t.Run("normalize description", func(t *testing.T) {
		// é can be represented as:
		// - Composed: U+00E9 (single character)
		// - Decomposed: U+0065 U+0301 (e + combining acute)
		decomposed := "e\u0301" // e + combining acute

		schema := map[string]interface{}{
			"type": "string",
			"description": decomposed + " acute",
		}

		result := sanitizeJSONSchema(schema).(map[string]interface{})
		desc := result["description"].(string)

		// Should be normalized to composed form
		assert.NotEqual(t, decomposed + " acute", desc)
		// The normalized form should be shorter (composed)
		assert.True(t, len(desc) <= len(decomposed + " acute"))
	})

	t.Run("normalize property names and descriptions", func(t *testing.T) {
		decomposed := "cafe\u0301" // café with decomposed accent

		schema := map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"name": map[string]interface{}{
					"type": "string",
					"description": decomposed,
				},
			},
		}

		result := sanitizeJSONSchema(schema).(map[string]interface{})
		props := result["properties"].(map[string]interface{})
		name := props["name"].(map[string]interface{})
		desc := name["description"].(string)

		// Description should be normalized
		assert.NotEqual(t, decomposed, desc)
	})

	t.Run("already normalized strings unchanged", func(t *testing.T) {
		normalized := "café" // Already in NFC form (U+00E9)

		schema := map[string]interface{}{
			"type": "string",
			"description": normalized,
		}

		result := sanitizeJSONSchema(schema).(map[string]interface{})
		desc := result["description"].(string)

		// Should remain the same
		assert.Equal(t, normalized, desc)
	})

	t.Run("ASCII strings unchanged", func(t *testing.T) {
		ascii := "Hello World"

		schema := map[string]interface{}{
			"type": "string",
			"description": ascii,
		}

		result := sanitizeJSONSchema(schema).(map[string]interface{})
		desc := result["description"].(string)

		assert.Equal(t, ascii, desc)
	})
}

// TestSanitizeJSONSchema_RealWorldExamples tests real-world MCP schema patterns
func TestSanitizeJSONSchema_RealWorldExamples(t *testing.T) {
	t.Run("MCP tool with missing type", func(t *testing.T) {
		// Real example from an MCP server
		schema := map[string]interface{}{
			"properties": map[string]interface{}{
				"query": map[string]interface{}{
					"description": "Search query",
				},
			},
			"required": []interface{}{"query"},
		}

		result := sanitizeJSONSchema(schema).(map[string]interface{})

		// Should infer object type
		assert.Equal(t, "object", result["type"])

		// Should infer query as string
		props := result["properties"].(map[string]interface{})
		query := props["query"].(map[string]interface{})
		assert.Equal(t, "string", query["type"])
	})

	t.Run("weather API schema", func(t *testing.T) {
		schema := map[string]interface{}{
			"properties": map[string]interface{}{
				"location": map[string]interface{}{
					"description": "City name",
				},
				"units": map[string]interface{}{
					"enum": []interface{}{"celsius", "fahrenheit"},
				},
			},
			"required": []interface{}{"location"},
		}

		result := sanitizeJSONSchema(schema).(map[string]interface{})

		assert.Equal(t, "object", result["type"])

		props := result["properties"].(map[string]interface{})
		location := props["location"].(map[string]interface{})
		units := props["units"].(map[string]interface{})

		assert.Equal(t, "string", location["type"])
		assert.Equal(t, "string", units["type"])
	})

	t.Run("file system tool with nested objects", func(t *testing.T) {
		schema := map[string]interface{}{
			"properties": map[string]interface{}{
				"path": map[string]interface{}{
					"type": "string",
				},
				"options": map[string]interface{}{
					"properties": map[string]interface{}{
						"recursive": map[string]interface{}{
							"type": "boolean",
						},
						"maxDepth": map[string]interface{}{
							"type": "integer",
						},
					},
				},
			},
		}

		result := sanitizeJSONSchema(schema).(map[string]interface{})

		assert.Equal(t, "object", result["type"])

		props := result["properties"].(map[string]interface{})
		options := props["options"].(map[string]interface{})

		assert.Equal(t, "object", options["type"])

		optProps := options["properties"].(map[string]interface{})
		maxDepth := optProps["maxDepth"].(map[string]interface{})
		assert.Equal(t, "number", maxDepth["type"]) // integer → number
	})

	t.Run("array parameter without items", func(t *testing.T) {
		schema := map[string]interface{}{
			"properties": map[string]interface{}{
				"tags": map[string]interface{}{
					"type": "array",
				},
			},
		}

		result := sanitizeJSONSchema(schema).(map[string]interface{})
		props := result["properties"].(map[string]interface{})
		tags := props["tags"].(map[string]interface{})

		// Should add default items
		assert.NotNil(t, tags["items"])
		items := tags["items"].(map[string]interface{})
		assert.Equal(t, "string", items["type"])
	})
}

// TestSanitizeSchemaJSON tests the JSON convenience function
func TestSanitizeSchemaJSON(t *testing.T) {
	t.Run("valid JSON schema", func(t *testing.T) {
		input := `{
			"properties": {
				"name": {"description": "User name"}
			}
		}`

		output, err := SanitizeSchemaJSON(input)
		require.NoError(t, err)

		var result map[string]interface{}
		err = json.Unmarshal([]byte(output), &result)
		require.NoError(t, err)

		assert.Equal(t, "object", result["type"])

		props := result["properties"].(map[string]interface{})
		name := props["name"].(map[string]interface{})
		assert.Equal(t, "string", name["type"])
	})

	t.Run("invalid JSON", func(t *testing.T) {
		input := `{invalid json`

		_, err := SanitizeSchemaJSON(input)
		assert.Error(t, err)
	})

	t.Run("boolean schema", func(t *testing.T) {
		input := `true`

		output, err := SanitizeSchemaJSON(input)
		require.NoError(t, err)

		var result map[string]interface{}
		err = json.Unmarshal([]byte(output), &result)
		require.NoError(t, err)

		assert.Equal(t, "string", result["type"])
	})
}

// TestSanitizeJSONSchema_EdgeCases tests edge cases and corner scenarios
func TestSanitizeJSONSchema_EdgeCases(t *testing.T) {
	t.Run("empty schema object", func(t *testing.T) {
		schema := map[string]interface{}{}

		result := sanitizeJSONSchema(schema).(map[string]interface{})

		// Should default to string
		assert.Equal(t, "string", result["type"])
	})

	t.Run("null values preserved", func(t *testing.T) {
		schema := map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"optional": nil,
			},
		}

		result := sanitizeJSONSchema(schema).(map[string]interface{})
		props := result["properties"].(map[string]interface{})

		// nil should be preserved (though it's unusual)
		assert.Contains(t, props, "optional")
	})

	t.Run("numbers preserved", func(t *testing.T) {
		schema := map[string]interface{}{
			"type": "number",
			"minimum": 0,
			"maximum": 100,
		}

		result := sanitizeJSONSchema(schema).(map[string]interface{})

		assert.Equal(t, "number", result["type"])
		assert.Equal(t, 0, result["minimum"])
		assert.Equal(t, 100, result["maximum"])
	})

	t.Run("deeply nested schemas", func(t *testing.T) {
		schema := map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"level1": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"level2": map[string]interface{}{
							"type": "object",
							"properties": map[string]interface{}{
								"level3": map[string]interface{}{
									"type": "integer",
								},
							},
						},
					},
				},
			},
		}

		result := sanitizeJSONSchema(schema).(map[string]interface{})

		// Navigate to level3
		props1 := result["properties"].(map[string]interface{})
		level1 := props1["level1"].(map[string]interface{})
		props2 := level1["properties"].(map[string]interface{})
		level2 := props2["level2"].(map[string]interface{})
		props3 := level2["properties"].(map[string]interface{})
		level3 := props3["level3"].(map[string]interface{})

		// Integer should be normalized to number
		assert.Equal(t, "number", level3["type"])
	})

	t.Run("array in array", func(t *testing.T) {
		schema := map[string]interface{}{
			"type": "array",
			"items": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "integer",
				},
			},
		}

		result := sanitizeJSONSchema(schema).(map[string]interface{})

		assert.Equal(t, "array", result["type"])

		items1 := result["items"].(map[string]interface{})
		assert.Equal(t, "array", items1["type"])

		items2 := items1["items"].(map[string]interface{})
		assert.Equal(t, "number", items2["type"]) // integer → number
	})

	t.Run("mixed type union defaults to first", func(t *testing.T) {
		schema := map[string]interface{}{
			"type": []interface{}{"unsupported", "string", "number"},
		}

		result := sanitizeJSONSchema(schema).(map[string]interface{})

		// Should pick first supported type
		assert.Equal(t, "string", result["type"])
	})

	t.Run("all unsupported types in union", func(t *testing.T) {
		schema := map[string]interface{}{
			"type": []interface{}{"unsupported1", "unsupported2"},
			"description": "some field",
		}

		result := sanitizeJSONSchema(schema).(map[string]interface{})

		// Should fall back to inference, which defaults to string
		assert.Equal(t, "string", result["type"])
	})
}

// TestNormalizeUnicode tests the Unicode normalization function
func TestNormalizeUnicode(t *testing.T) {
	t.Run("decomposed to composed", func(t *testing.T) {
		// é as e + combining acute
		decomposed := "e\u0301"
		normalized := normalizeUnicode(decomposed)

		// Should normalize to composed form é (U+00E9)
		assert.Equal(t, "\u00e9", normalized)
	})

	t.Run("already composed unchanged", func(t *testing.T) {
		composed := "\u00e9" // é in composed form
		normalized := normalizeUnicode(composed)

		assert.Equal(t, composed, normalized)
	})

	t.Run("ASCII unchanged", func(t *testing.T) {
		ascii := "Hello World 123"
		normalized := normalizeUnicode(ascii)

		assert.Equal(t, ascii, normalized)
	})

	t.Run("mixed ASCII and Unicode", func(t *testing.T) {
		mixed := "Hello e\u0301" // Hello é (decomposed)
		normalized := normalizeUnicode(mixed)

		assert.Equal(t, "Hello \u00e9", normalized)
	})

	t.Run("invalid UTF-8 preserved", func(t *testing.T) {
		// Create invalid UTF-8 by concatenating byte values
		invalid := string([]byte{0xFF, 0xFE, 0xFD})
		normalized := normalizeUnicode(invalid)

		// Should return as-is
		assert.Equal(t, invalid, normalized)
	})
}

// TestHasAny tests the hasAny helper function
func TestHasAny(t *testing.T) {
	t.Run("has one key", func(t *testing.T) {
		m := map[string]interface{}{
			"foo": "bar",
		}

		assert.True(t, hasAny(m, "foo"))
		assert.True(t, hasAny(m, "foo", "baz"))
		assert.False(t, hasAny(m, "baz"))
	})

	t.Run("has multiple keys", func(t *testing.T) {
		m := map[string]interface{}{
			"foo": "bar",
			"baz": "qux",
		}

		assert.True(t, hasAny(m, "foo", "baz"))
		assert.True(t, hasAny(m, "foo"))
		assert.True(t, hasAny(m, "baz"))
		assert.False(t, hasAny(m, "missing"))
	})

	t.Run("empty map", func(t *testing.T) {
		m := map[string]interface{}{}

		assert.False(t, hasAny(m, "foo"))
		assert.False(t, hasAny(m, "foo", "bar"))
	})
}

// TestSanitizeJSONSchema_PerformanceConsiderations tests performance with large schemas
func TestSanitizeJSONSchema_PerformanceConsiderations(t *testing.T) {
	t.Run("large schema with many properties", func(t *testing.T) {
		// Create a schema with 100 properties
		props := make(map[string]interface{}, 100)
		for i := 0; i < 100; i++ {
			props[string(rune('a'+i%26))+string(rune('0'+i/26))] = map[string]interface{}{
				"type": "string",
			}
		}

		schema := map[string]interface{}{
			"type": "object",
			"properties": props,
		}

		result := sanitizeJSONSchema(schema).(map[string]interface{})

		assert.Equal(t, "object", result["type"])
		resultProps := result["properties"].(map[string]interface{})
		assert.Len(t, resultProps, 100)
	})

	t.Run("deeply nested schema", func(t *testing.T) {
		// Create a schema nested 10 levels deep
		schema := map[string]interface{}{
			"type": "integer",
		}

		for i := 0; i < 10; i++ {
			schema = map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"nested": schema,
				},
			}
		}

		result := sanitizeJSONSchema(schema).(map[string]interface{})

		// Verify it completes without stack overflow
		assert.Equal(t, "object", result["type"])

		// Navigate down and verify integer was normalized
		current := result
		for i := 0; i < 10; i++ {
			props := current["properties"].(map[string]interface{})
			current = props["nested"].(map[string]interface{})
		}
		assert.Equal(t, "number", current["type"]) // integer → number
	})
}
