package cli

import (
	"errors"
	"os"
	"path/filepath"
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

func TestParseProgramArgs_empty(t *testing.T) {
	t.Run("nil OnDefault", func(t *testing.T) {
		code, err := parseProgramArgs(nil, ParseOptions{})
		if !errors.Is(err, errMissingCommand) {
			t.Fatalf("err = %v, want errMissingCommand", err)
		}
		if code != ExitUsage {
			t.Fatalf("code = %d, want %d", code, ExitUsage)
		}
	})
	t.Run("OnDefault runs", func(t *testing.T) {
		var ran bool
		code, err := parseProgramArgs(nil, ParseOptions{
			OnDefault: func() { ran = true },
		})
		if err != nil {
			t.Fatalf("err = %v, want nil", err)
		}
		if code != 0 {
			t.Fatalf("code = %d, want 0", code)
		}
		if !ran {
			t.Fatal("OnDefault was not called")
		}
	})
}

func TestParseProgramArgs_help_version(t *testing.T) {
	for _, argv := range [][]string{
		{"help"},
		{"--help"},
		{"-h"},
		{"version"},
		{"--version"},
		{"-v"},
	} {
		t.Run(argv[0], func(t *testing.T) {
			code, err := parseProgramArgs(argv, ParseOptions{})
			if err != nil {
				t.Fatalf("parseProgramArgs(%v) = %v", argv, err)
			}
			if code != 0 {
				t.Fatalf("code = %d, want 0", code)
			}
		})
	}
}

func TestParseProgramArgs_list(t *testing.T) {
	t.Run("nil OnList", func(t *testing.T) {
		code, err := parseProgramArgs([]string{"list"}, ParseOptions{})
		if !errors.Is(err, errMissingCommand) {
			t.Fatalf("err = %v, want errMissingCommand", err)
		}
		if code != ExitUsage {
			t.Fatalf("code = %d, want %d", code, ExitUsage)
		}
	})
	t.Run("OnList runs", func(t *testing.T) {
		var ran bool
		code, err := parseProgramArgs([]string{"list"}, ParseOptions{
			OnList: func() { ran = true },
		})
		if err != nil {
			t.Fatalf("err = %v", err)
		}
		if code != 0 {
			t.Fatalf("code = %d, want 0", code)
		}
		if !ran {
			t.Fatal("OnList was not called")
		}
	})
}

func TestParseProgramArgs_run(t *testing.T) {
	tmp := t.TempDir()
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}

	t.Run("captures inferred name and command", func(t *testing.T) {
		var gotName string
		var gotArgs []string
		code, err := parseProgramArgs([]string{"run", "npm", "start"}, ParseOptions{
			OnRun: func(name string, cmdArgs []string) (int, error) {
				gotName, gotArgs = name, append([]string(nil), cmdArgs...)
				return 0, nil
			},
		})
		if err != nil {
			t.Fatal(err)
		}
		if code != 0 {
			t.Fatalf("code = %d, want 0", code)
		}
		wantName := filepath.Base(tmp)
		if gotName != wantName {
			t.Errorf("name = %q, want %q", gotName, wantName)
		}
		wantArgs := []string{"npm", "start"}
		if len(gotArgs) != len(wantArgs) {
			t.Fatalf("cmdArgs = %v, want %v", gotArgs, wantArgs)
		}
		for i := range wantArgs {
			if gotArgs[i] != wantArgs[i] {
				t.Errorf("cmdArgs[%d] = %q, want %q", i, gotArgs[i], wantArgs[i])
			}
		}
	})

	t.Run("OnRun exit code propagates", func(t *testing.T) {
		code, err := parseProgramArgs([]string{"run", "sh", "-c", "true"}, ParseOptions{
			OnRun: func(string, []string) (int, error) { return 42, nil },
		})
		if err != nil {
			t.Fatal(err)
		}
		if code != 42 {
			t.Fatalf("code = %d, want 42", code)
		}
	})

	t.Run("--name", func(t *testing.T) {
		var gotName string
		code, err := parseProgramArgs([]string{"run", "--name", "myapp", "sh", "-c", "true"}, ParseOptions{
			OnRun: func(name string, cmdArgs []string) (int, error) {
				gotName = name
				return 0, nil
			},
		})
		if err != nil {
			t.Fatal(err)
		}
		if code != 0 {
			t.Fatalf("code = %d, want 0", code)
		}
		if gotName != "myapp" {
			t.Errorf("name = %q, want myapp", gotName)
		}
	})

	t.Run("missing command after run", func(t *testing.T) {
		code, err := parseProgramArgs([]string{"run", "--name", "x"}, ParseOptions{
			OnRun: func(string, []string) (int, error) { return 0, nil },
		})
		if err == nil {
			t.Fatal("expected error")
		}
		if code != ExitUsage {
			t.Fatalf("code = %d, want %d", code, ExitUsage)
		}
	})

	t.Run("run requires subcommand token", func(t *testing.T) {
		code, err := parseProgramArgs([]string{"run"}, ParseOptions{})
		if err == nil {
			t.Fatal("expected error")
		}
		if code != ExitUsage {
			t.Fatalf("code = %d, want %d", code, ExitUsage)
		}
	})

	t.Run("nil OnRun", func(t *testing.T) {
		code, err := parseProgramArgs([]string{"run", "npm", "start"}, ParseOptions{})
		if !errors.Is(err, errMissingCommand) {
			t.Fatalf("err = %v, want errMissingCommand", err)
		}
		if code != ExitUsage {
			t.Fatalf("code = %d, want %d", code, ExitUsage)
		}
	})
}

func TestParseProgramArgs_named(t *testing.T) {
	var gotName string
	var gotArgs []string
	code, err := parseProgramArgs([]string{"svc", "npm", "test"}, ParseOptions{
		OnRun: func(name string, cmdArgs []string) (int, error) {
			gotName, gotArgs = name, append([]string(nil), cmdArgs...)
			return 0, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if code != 0 {
		t.Fatalf("code = %d, want 0", code)
	}
	if gotName != "svc" {
		t.Errorf("name = %q, want svc", gotName)
	}
	if len(gotArgs) != 2 || gotArgs[0] != "npm" || gotArgs[1] != "test" {
		t.Errorf("cmdArgs = %v, want [npm test]", gotArgs)
	}
}

func TestParseProgramArgs_named_usage(t *testing.T) {
	code, err := parseProgramArgs([]string{"onlyname"}, ParseOptions{
		OnRun: func(string, []string) (int, error) { return 0, nil },
	})
	if err == nil {
		t.Fatal("expected usage error")
	}
	if code != ExitUsage {
		t.Fatalf("code = %d, want %d", code, ExitUsage)
	}
}

func TestParseProgramArgs_unknown_first_token(t *testing.T) {
	// Single token that is not a known subcommand → named mode, needs cmd
	code, err := parseProgramArgs([]string{"foo"}, ParseOptions{})
	if err == nil {
		t.Fatal("expected error")
	}
	if code != ExitUsage {
		t.Fatalf("code = %d, want %d", code, ExitUsage)
	}
}
