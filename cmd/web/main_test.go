package main

import (
	"testing"

	"github.com/gin-gonic/gin"
)

func TestGinMode(t *testing.T) {
	t.Setenv(gin.EnvGinMode, "")
	if got := ginMode(); got != gin.ReleaseMode {
		t.Errorf("ginMode() without GIN_MODE = %q, want %q", got, gin.ReleaseMode)
	}
	t.Setenv(gin.EnvGinMode, gin.DebugMode)
	if got := ginMode(); got != gin.DebugMode {
		t.Errorf("ginMode() with GIN_MODE=debug = %q, want %q", got, gin.DebugMode)
	}
}
