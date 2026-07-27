package patch

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEscapeJSONPointer(t *testing.T) {
	result := EscapeJSONPointer("key~with/slash")
	assert.Equal(t, "key~0with~1slash", result)
}

func TestLabelPath(t *testing.T) {
	path := LabelPath("example.com/key")
	assert.Equal(t, "/metadata/labels/example.com~1key", path)
}

func TestAnnotationPath(t *testing.T) {
	path := AnnotationPath("example.com/annotation")
	assert.Equal(t, "/metadata/annotations/example.com~1annotation", path)
}

func TestNew(t *testing.T) {
	t.Run("creates builder with no operations", func(t *testing.T) {
		builder := New()
		assert.NotNil(t, builder)
		assert.Empty(t, builder.operations)
	})

	t.Run("creates builder with options", func(t *testing.T) {
		builder := New(
			WithAdd("/metadata/labels/key", "value"),
			WithReplace("/spec/replicas", 3),
		)
		assert.NotNil(t, builder)
		assert.Len(t, builder.operations, 2)
	})
}

func TestWithAdd(t *testing.T) {
	t.Run("adds operation with string value", func(t *testing.T) {
		builder := New(WithAdd("/metadata/labels/env", "production"))

		assert.Len(t, builder.operations, 1)
		assert.Equal(t, "add", builder.operations[0].Op)
		assert.Equal(t, "/metadata/labels/env", builder.operations[0].Path)
		assert.Equal(t, "production", builder.operations[0].Value)
	})

	t.Run("adds operation with numeric value", func(t *testing.T) {
		builder := New(WithAdd("/spec/replicas", 5))

		assert.Len(t, builder.operations, 1)
		assert.Equal(t, "add", builder.operations[0].Op)
		assert.Equal(t, "/spec/replicas", builder.operations[0].Path)
		assert.Equal(t, 5, builder.operations[0].Value)
	})

	t.Run("adds operation with map value", func(t *testing.T) {
		labels := map[string]string{"key": "value"}
		builder := New(WithAdd("/metadata/labels", labels))

		assert.Len(t, builder.operations, 1)
		assert.Equal(t, "add", builder.operations[0].Op)
		assert.Equal(t, "/metadata/labels", builder.operations[0].Path)
		assert.Equal(t, labels, builder.operations[0].Value)
	})
}

func TestWithReplace(t *testing.T) {
	t.Run("replaces with string value", func(t *testing.T) {
		builder := New(WithReplace("/spec/template/spec/image", "nginx:latest"))

		assert.Len(t, builder.operations, 1)
		assert.Equal(t, "replace", builder.operations[0].Op)
		assert.Equal(t, "/spec/template/spec/image", builder.operations[0].Path)
		assert.Equal(t, "nginx:latest", builder.operations[0].Value)
	})

	t.Run("replaces with boolean value", func(t *testing.T) {
		builder := New(WithReplace("/spec/running", true))

		assert.Len(t, builder.operations, 1)
		assert.Equal(t, "replace", builder.operations[0].Op)
		assert.Equal(t, "/spec/running", builder.operations[0].Path)
		assert.Equal(t, true, builder.operations[0].Value)
	})
}

func TestWithRemove(t *testing.T) {
	t.Run("removes operation without value", func(t *testing.T) {
		builder := New(WithRemove("/metadata/annotations/old-key"))

		assert.Len(t, builder.operations, 1)
		assert.Equal(t, "remove", builder.operations[0].Op)
		assert.Equal(t, "/metadata/annotations/old-key", builder.operations[0].Path)
		assert.Nil(t, builder.operations[0].Value)
	})
}

func TestGeneratePayload(t *testing.T) {
	t.Run("returns error for empty operations", func(t *testing.T) {
		builder := New()
		payload, err := builder.GeneratePayload()

		assert.Nil(t, payload)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no patch operations defined")
	})

	t.Run("generates valid JSON for single operation", func(t *testing.T) {
		builder := New(WithAdd("/metadata/labels/app", "myapp"))
		payload, err := builder.GeneratePayload()

		assert.NoError(t, err)
		assert.NotNil(t, payload)

		var operations []PatchOperation
		err = json.Unmarshal(payload, &operations)
		assert.NoError(t, err)
		assert.Len(t, operations, 1)

		assert.Equal(t, "add", operations[0].Op)
		assert.Equal(t, "/metadata/labels/app", operations[0].Path)
		assert.Equal(t, "myapp", operations[0].Value)
	})

	t.Run("generates valid JSON for multiple operations", func(t *testing.T) {
		builder := New(
			WithAdd("/metadata/labels/env", "prod"),
			WithReplace("/spec/replicas", float64(3)),
			WithRemove("/metadata/annotations/deprecated"),
		)
		payload, err := builder.GeneratePayload()

		assert.NoError(t, err)
		assert.NotNil(t, payload)

		var operations []PatchOperation
		err = json.Unmarshal(payload, &operations)
		assert.NoError(t, err)
		assert.Len(t, operations, 3)

		assert.Equal(t, "add", operations[0].Op)
		assert.Equal(t, "/metadata/labels/env", operations[0].Path)
		assert.Equal(t, "prod", operations[0].Value)

		assert.Equal(t, "replace", operations[1].Op)
		assert.Equal(t, "/spec/replicas", operations[1].Path)
		assert.Equal(t, float64(3), operations[1].Value)

		assert.Equal(t, "remove", operations[2].Op)
		assert.Equal(t, "/metadata/annotations/deprecated", operations[2].Path)
		assert.Nil(t, operations[2].Value)
	})

	t.Run("generates valid JSON with complex value", func(t *testing.T) {
		complexValue := map[string]interface{}{
			"cpu":    "2",
			"memory": "4Gi",
		}
		builder := New(WithReplace("/spec/resources", complexValue))
		payload, err := builder.GeneratePayload()

		assert.NoError(t, err)
		assert.NotNil(t, payload)

		var operations []PatchOperation
		err = json.Unmarshal(payload, &operations)
		assert.NoError(t, err)
		assert.Len(t, operations, 1)

		assert.Equal(t, "replace", operations[0].Op)
		assert.Equal(t, "/spec/resources", operations[0].Path)

		valueMap, ok := operations[0].Value.(map[string]interface{})
		assert.True(t, ok)
		assert.Equal(t, "2", valueMap["cpu"])
		assert.Equal(t, "4Gi", valueMap["memory"])
	})
}

func TestBuilderChaining(t *testing.T) {
	t.Run("can chain multiple options", func(t *testing.T) {
		builder := New(
			WithAdd("/metadata/labels/new", "label"),
			WithReplace("/metadata/labels/existing", "updated"),
			WithRemove("/metadata/labels/old"),
			WithAdd("/spec/replicas", float64(5)),
		)

		assert.Len(t, builder.operations, 4)

		payload, err := builder.GeneratePayload()
		assert.NoError(t, err)

		var operations []PatchOperation
		err = json.Unmarshal(payload, &operations)
		assert.NoError(t, err)
		assert.Len(t, operations, 4)
	})
}

func TestWithAddLabel(t *testing.T) {
	t.Run("creates labels map when nil", func(t *testing.T) {
		builder := New(WithAddLabel("app", "myapp", nil))
		assert.Equal(t, "add", builder.operations[0].Op)
		assert.Equal(t, "/metadata/labels", builder.operations[0].Path)
		assert.Equal(t, map[string]string{"app": "myapp"}, builder.operations[0].Value)
	})

	t.Run("adds specific label when labels exist", func(t *testing.T) {
		existingLabels := map[string]string{"existing": "value"}
		builder := New(WithAddLabel("app", "myapp", existingLabels))
		assert.Equal(t, "add", builder.operations[0].Op)
		assert.Equal(t, "/metadata/labels/app", builder.operations[0].Path)
		assert.Equal(t, "myapp", builder.operations[0].Value)
	})
}

func TestWithAddAnnotation(t *testing.T) {
	t.Run("creates annotations map when nil", func(t *testing.T) {
		builder := New(WithAddAnnotation("desc", "test", nil))
		assert.Equal(t, "add", builder.operations[0].Op)
		assert.Equal(t, "/metadata/annotations", builder.operations[0].Path)
		assert.Equal(t, map[string]string{"desc": "test"}, builder.operations[0].Value)
	})

	t.Run("adds specific annotation when annotations exist", func(t *testing.T) {
		existingAnnotations := map[string]string{"existing": "value"}
		builder := New(WithAddAnnotation("desc", "test", existingAnnotations))
		assert.Equal(t, "add", builder.operations[0].Op)
		assert.Equal(t, "/metadata/annotations/desc", builder.operations[0].Path)
		assert.Equal(t, "test", builder.operations[0].Value)
	})
}
