package src

import (
	"testing"
)

func TestInferName(t *testing.T) {
	tests := []struct {
		dir  string
		want string
	}{
		{"/home/user/myproject", "myproject"},
		{"/home/user/myproject/", "myproject"},
		{"/home/user/myproject/subdir", "subdir"},
	}

	for _, tt := range tests {
		got := inferName(tt.dir)
		if got != tt.want {
			t.Errorf("inferName(%q) = %q, want %q", tt.dir, got, tt.want)
		}
	}
}

func TestRunCommandWithName(t *testing.T) {
	tests := []struct {
		name    string
		cmdArgs []string
		wantErr bool
	}{
		{"myapp", []string{"npm", "start"}, false},
		{"myapp", []string{}, true},
		{"myapp", nil, true},
	}

	for _, tt := range tests {
		err := runCommandWithName(tt.name, tt.cmdArgs)
		if (err != nil) != tt.wantErr {
			t.Errorf("runCommandWithName(%q, %v) error = %v, wantErr %v",
				tt.name, tt.cmdArgs, err, tt.wantErr)
		}
	}
}
