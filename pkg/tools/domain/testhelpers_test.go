package domain_test

import (
	"context"
)

type mockSippy struct {
	responses map[string][]byte
}

func newMockSippy(responses map[string][]byte) *mockSippy {
	return &mockSippy{responses: responses}
}

func (m *mockSippy) Get(ctx context.Context, path string, params map[string]string) ([]byte, error) {
	if data, ok := m.responses[path]; ok {
		return data, nil
	}
	return []byte(`{}`), nil
}

type capturingSippy struct {
	response []byte
	onGet    func(path string, params map[string]string)
}

func (m *capturingSippy) Get(ctx context.Context, path string, params map[string]string) ([]byte, error) {
	if m.onGet != nil {
		m.onGet(path, params)
	}
	return m.response, nil
}
