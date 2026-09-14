package harness

import "testing"

func TestNewProviderUnknown(t *testing.T) {
	_, err := NewProvider(&Config{Provider: "nope"})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestNewProviderNilConfig(t *testing.T) {
	_, err := NewProvider(nil)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestNewProviderOllama(t *testing.T) {
	p, err := NewProvider(&Config{Provider: "ollama"})
	if err != nil {
		t.Fatal(err)
	}
	if p == nil {
		t.Fatal("nil provider")
	}
}
