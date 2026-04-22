package provider

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
)

type DNSProvider interface {
	Name() string
	Validate(context.Context) (*ValidationResult, error)
	ListDomains(context.Context) ([]Domain, error)
	ListRecords(context.Context, string) ([]DNSRecord, error)
	UpsertRecord(context.Context, string, RecordMutation) (*DNSRecord, error)
	DeleteRecord(context.Context, string, string) error
	ExportConfig() map[string]any
}

type Factory func(config map[string]any) (DNSProvider, error)

type FieldType string

const (
	FieldTypeText     FieldType = "text"
	FieldTypePassword FieldType = "password"
	FieldTypeNumber   FieldType = "number"
	FieldTypeBoolean  FieldType = "boolean"
)

type FieldSpec struct {
	Key          string    `json:"key"`
	Label        string    `json:"label"`
	Type         FieldType `json:"type"`
	Required     bool      `json:"required"`
	Placeholder  string    `json:"placeholder,omitempty"`
	HelpText     string    `json:"helpText,omitempty"`
	DefaultValue any       `json:"defaultValue,omitempty"`
}

type Descriptor struct {
	Key          string         `json:"key"`
	Label        string         `json:"label"`
	Description  string         `json:"description,omitempty"`
	Fields       []FieldSpec    `json:"fields"`
	SampleConfig map[string]any `json:"sampleConfig,omitempty"`
}

var (
	registryMu  sync.RWMutex
	registry    = map[string]Factory{}
	descriptors = map[string]Descriptor{}
)

func Register(name string, factory Factory) error {
	trimmed := strings.ToLower(strings.TrimSpace(name))
	if trimmed == "" {
		return fmt.Errorf("provider name is required")
	}
	if factory == nil {
		return fmt.Errorf("provider factory is required")
	}
	registryMu.Lock()
	defer registryMu.Unlock()
	if _, exists := registry[trimmed]; exists {
		return fmt.Errorf("provider %s already registered", trimmed)
	}
	registry[trimmed] = factory
	return nil
}

func MustRegister(name string, factory Factory) {
	if err := Register(name, factory); err != nil {
		panic(err)
	}
}

func New(name string, config map[string]any) (DNSProvider, error) {
	trimmed := strings.ToLower(strings.TrimSpace(name))
	registryMu.RLock()
	factory, ok := registry[trimmed]
	registryMu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("provider %s is not supported yet", trimmed)
	}
	return factory(config)
}

func RegisteredProviders() []string {
	registryMu.RLock()
	defer registryMu.RUnlock()
	items := make([]string, 0, len(registry))
	for name := range registry {
		items = append(items, name)
	}
	sort.Strings(items)
	return items
}

func RegisterDescriptor(descriptor Descriptor) error {
	trimmed := strings.ToLower(strings.TrimSpace(descriptor.Key))
	if trimmed == "" {
		return fmt.Errorf("provider descriptor key is required")
	}
	if strings.TrimSpace(descriptor.Label) == "" {
		return fmt.Errorf("provider descriptor label is required")
	}
	descriptor.Key = trimmed
	registryMu.Lock()
	defer registryMu.Unlock()
	descriptors[trimmed] = descriptor
	return nil
}

func MustRegisterDescriptor(descriptor Descriptor) {
	if err := RegisterDescriptor(descriptor); err != nil {
		panic(err)
	}
}

func RegisteredDescriptors() []Descriptor {
	registryMu.RLock()
	defer registryMu.RUnlock()
	keys := make([]string, 0, len(descriptors))
	for key := range descriptors {
		if _, ok := registry[key]; ok {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	items := make([]Descriptor, 0, len(keys))
	for _, key := range keys {
		items = append(items, descriptors[key])
	}
	return items
}
