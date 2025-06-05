package cache

import "testing"

func TestGenerateCacheKey(t *testing.T) {
	tests := []struct {
		name         string
		serviceName  string
		objectType   string
		identifier   string
		paramsKey    []string
		expectedKey  string
	}{
		{
			name:        "without paramsKey",
			serviceName: "user",
			objectType:  "profile",
			identifier:  "123",
			paramsKey:   nil,
			expectedKey: "quizbyte:user:profile:123",
		},
		{
			name:        "with empty paramsKey",
			serviceName: "user",
			objectType:  "profile",
			identifier:  "123",
			paramsKey:   []string{},
			expectedKey: "quizbyte:user:profile:123",
		},
		{
			name:        "with one paramsKey",
			serviceName: "product",
			objectType:  "details",
			identifier:  "abc",
			paramsKey:   []string{"param1"},
			expectedKey: "quizbyte:product:details:abc:param1",
		},
		{
			name:        "with multiple paramsKey",
			serviceName: "order",
			objectType:  "item",
			identifier:  "xyz",
			paramsKey:   []string{"param1", "param2", "param3"},
			expectedKey: "quizbyte:order:item:xyz:param1_param2_param3",
		},
		{
			name:        "with paramsKey containing special characters",
			serviceName: "service",
			objectType:  "type",
			identifier:  "id",
			paramsKey:   []string{"param-1", "param_2"},
			expectedKey: "quizbyte:service:type:id:param-1_param_2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actualKey := GenerateCacheKey(tt.serviceName, tt.objectType, tt.identifier, tt.paramsKey...)
			if actualKey != tt.expectedKey {
				t.Errorf("GenerateCacheKey() = %v, want %v", actualKey, tt.expectedKey)
			}
		})
	}
}
