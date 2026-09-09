package claude

import (
	"strings"
	"testing"
)

func TestEvaluateFallback(t *testing.T) {
	tests := []struct {
		name               string
		reply              string
		contextBlock       string
		expectedIsFallback bool
		expectedContains   string
	}{
		{
			name:               "Clear fallback due to missing knowledge",
			reply:              "Dạ câu hỏi này nằm ngoài lĩnh vực chuyên môn của em.",
			contextBlock:       "Some context",
			expectedIsFallback: true,
			expectedContains:   "chuyên viên CSKH của Đông Đô sẽ trực tiếp tham gia",
		},
		{
			name:               "Fallback due to empty contextBlock",
			reply:              "Tôi là trợ lý ảo Đông Đô.",
			contextBlock:       "",
			expectedIsFallback: true,
			expectedContains:   "chuyên viên CSKH của Đông Đô sẽ trực tiếp tham gia",
		},
		{
			name:               "Normal valid answer with context",
			reply:              "Dạ, bước giá (tick size) là mức thay đổi giá nhỏ nhất có thể của một hợp đồng.",
			contextBlock:       "Tài liệu bước giá...",
			expectedIsFallback: false,
			expectedContains:   "bước giá",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			reply, isFallback := evaluateFallback(tc.reply, tc.contextBlock)
			if isFallback != tc.expectedIsFallback {
				t.Fatalf("expected isFallback=%v, got %v for test '%s'", tc.expectedIsFallback, isFallback, tc.name)
			}
			if !strings.Contains(reply, tc.expectedContains) {
				t.Fatalf("expected reply to contain %q, but got %q", tc.expectedContains, reply)
			}
		})
	}
}
