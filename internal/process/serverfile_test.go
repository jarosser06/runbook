package process

import (
	"os"
	"path/filepath"
	"testing"

	"runbookmcp.dev/internal/dirs"
)

func TestWriteAndReadServerFile(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir) // Go 1.24+: restores original CWD when the test ends

	data := ServerFileData{Addr: "http://localhost:8080", PID: 12345}
	if err := WriteServerFile(data); err != nil {
		t.Fatalf("WriteServerFile: %v", err)
	}

	// Verify the file exists in the expected location.
	if _, err := os.Stat(ServerRegistryFile); err != nil {
		t.Fatalf("server.json not found: %v", err)
	}

	got, err := ReadServerFile("")
	if err != nil {
		t.Fatalf("ReadServerFile: %v", err)
	}
	if got.Addr != data.Addr {
		t.Errorf("Addr = %q, want %q", got.Addr, data.Addr)
	}
	if got.PID != data.PID {
		t.Errorf("PID = %d, want %d", got.PID, data.PID)
	}
}

func TestReadServerFileWithWorkingDir(t *testing.T) {
	dir := t.TempDir()

	// Write into a specific directory.
	devToolsDir := filepath.Join(dir, dirs.StateDir)
	if err := os.MkdirAll(devToolsDir, 0755); err != nil {
		t.Fatal(err)
	}
	data := ServerFileData{Addr: "http://localhost:9999", PID: 99}
	b := []byte(`{"addr":"http://localhost:9999","pid":99}`)
	if err := os.WriteFile(filepath.Join(devToolsDir, "server.json"), b, 0644); err != nil {
		t.Fatal(err)
	}

	got, err := ReadServerFile(dir)
	if err != nil {
		t.Fatalf("ReadServerFile with workingDir: %v", err)
	}
	if got.Addr != data.Addr {
		t.Errorf("Addr = %q, want %q", got.Addr, data.Addr)
	}
	if got.PID != data.PID {
		t.Errorf("PID = %d, want %d", got.PID, data.PID)
	}
}

func TestReadServerFileMissing(t *testing.T) {
	dir := t.TempDir()
	_, err := ReadServerFile(dir)
	if err == nil {
		t.Error("expected error for missing server.json, got nil")
	}
}

func TestDeleteServerFile(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir) // Go 1.24+: restores original CWD when the test ends

	data := ServerFileData{Addr: "http://localhost:8080", PID: 1}
	if err := WriteServerFile(data); err != nil {
		t.Fatalf("WriteServerFile: %v", err)
	}

	DeleteServerFile("")
	if _, err := os.Stat(ServerRegistryFile); !os.IsNotExist(err) {
		t.Error("expected server.json to be deleted, but it still exists")
	}
}

func TestDeleteServerFileNoOp(t *testing.T) {
	// Should not panic if file doesn't exist.
	DeleteServerFile(t.TempDir())
}

func TestServerFilePathEmptyDir(t *testing.T) {
	got := serverFilePath("")
	if got != ServerRegistryFile {
		t.Errorf("serverFilePath(\"\") = %q, want %q", got, ServerRegistryFile)
	}
}

func TestServerFilePathWithDir(t *testing.T) {
	got := serverFilePath("/some/project")
	want := filepath.Join("/some/project", ServerRegistryFile)
	if got != want {
		t.Errorf("serverFilePath = %q, want %q", got, want)
	}
}

func TestIsProcessAliveCurrentProcess(t *testing.T) {
	pid := os.Getpid()
	if !IsProcessAlive(pid) {
		t.Errorf("IsProcessAlive(%d) = false for own PID", pid)
	}
}

func TestIsProcessAliveDeadPID(t *testing.T) {
	// PID 0 is never a real process; PID -1 is invalid.
	// Use a very large PID that is astronomically unlikely to exist.
	if IsProcessAlive(999999999) {
		t.Error("IsProcessAlive(999999999) = true, expected false for non-existent PID")
	}
}
