package cli

import "testing"

func TestParseRawParams(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want map[string]any
	}{
		{
			name: "empty",
			args: nil,
			want: map[string]any{},
		},
		{
			name: "equals form",
			args: []string{"--key=value"},
			want: map[string]any{"key": "value"},
		},
		{
			name: "single dash equals",
			args: []string{"-key=value"},
			want: map[string]any{"key": "value"},
		},
		{
			name: "space separated",
			args: []string{"--key", "value"},
			want: map[string]any{"key": "value"},
		},
		{
			name: "multiple params",
			args: []string{"--a=1", "--b=2"},
			want: map[string]any{"a": "1", "b": "2"},
		},
		{
			name: "flag followed by another flag (no value)",
			args: []string{"--flag", "--other=x"},
			want: map[string]any{"other": "x"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseRawParams(tt.args)
			if len(got) != len(tt.want) {
				t.Errorf("len = %d, want %d (got=%v, want=%v)", len(got), len(tt.want), got, tt.want)
				return
			}
			for k, wv := range tt.want {
				if got[k] != wv {
					t.Errorf("param[%q] = %v, want %v", k, got[k], wv)
				}
			}
		})
	}
}
