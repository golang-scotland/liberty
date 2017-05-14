package env

import (
	"testing"
)

func TestDefaultEnv(t *testing.T) {
	if Get() != Dev {
		t.Error("default unset env should default to 'dev'")
	}
}
