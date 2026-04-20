package patch

import (
	"encoding/json"
	"fmt"
	"strings"
)

type PatchOperation struct {
	Op    string      `json:"op"`
	Path  string      `json:"path"`
	Value interface{} `json:"value,omitempty"`
}

// ~ is encoded as ~0 and / is encoded as ~1 (RFC 6901)
func EscapeJSONPointer(s string) string {
	s = strings.ReplaceAll(s, "~", "~0")
	s = strings.ReplaceAll(s, "/", "~1")
	return s
}

func LabelPath(key string) string {
	return "/metadata/labels/" + EscapeJSONPointer(key)
}

func AnnotationPath(key string) string {
	return "/metadata/annotations/" + EscapeJSONPointer(key)
}

type Builder struct {
	operations []PatchOperation
}

type Option func(*Builder)

func New(opts ...Option) *Builder {
	b := &Builder{
		operations: []PatchOperation{},
	}
	for _, opt := range opts {
		opt(b)
	}
	return b
}

func WithAdd(path string, value interface{}) Option {
	return func(b *Builder) {
		b.operations = append(b.operations, PatchOperation{
			Op:    "add",
			Path:  path,
			Value: value,
		})
	}
}

func WithReplace(path string, value interface{}) Option {
	return func(b *Builder) {
		b.operations = append(b.operations, PatchOperation{
			Op:    "replace",
			Path:  path,
			Value: value,
		})
	}
}

func WithRemove(path string) Option {
	return func(b *Builder) {
		b.operations = append(b.operations, PatchOperation{
			Op:   "remove",
			Path: path,
		})
	}
}

func WithAddLabel(key, value string, existingLabels map[string]string) Option {
	if existingLabels == nil {
		return WithAdd("/metadata/labels", map[string]string{key: value})
	}
	return WithAdd(LabelPath(key), value)
}

func WithAddAnnotation(key, value string, existingAnnotations map[string]string) Option {
	if existingAnnotations == nil {
		return WithAdd("/metadata/annotations", map[string]string{key: value})
	}
	return WithAdd(AnnotationPath(key), value)
}

func (b *Builder) GeneratePayload() ([]byte, error) {
	if len(b.operations) == 0 {
		return nil, fmt.Errorf("no patch operations defined")
	}
	return json.Marshal(b.operations)
}
