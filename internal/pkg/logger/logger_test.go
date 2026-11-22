package logger

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"go.uber.org/zap/zapcore"
)

func TestSupportsColorFalseForRegularFile(t *testing.T) {
	f, err := os.Create(filepath.Join(t.TempDir(), "nohup.out"))
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	defer f.Close()
	if supportsColor(f) {
		t.Fatal("regular file should not support color")
	}
}

func TestLevelEncoderChoosesPlainTextOffTerminal(t *testing.T) {
	tests := []struct {
		format string
		color  bool
		want   zapcore.LevelEncoder
	}{
		{"json", true, zapcore.LowercaseLevelEncoder},
		{"console", true, zapcore.CapitalColorLevelEncoder},
		{"console", false, zapcore.CapitalLevelEncoder},
	}
	for _, tt := range tests {
		got := reflect.ValueOf(levelEncoder(tt.format, tt.color)).Pointer()
		want := reflect.ValueOf(tt.want).Pointer()
		if got != want {
			t.Fatalf("levelEncoder(%q, %v) = wrong encoder", tt.format, tt.color)
		}
	}
}
