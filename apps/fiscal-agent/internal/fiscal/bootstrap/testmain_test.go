package bootstrap_test

import (
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	_ = os.Setenv("FISCAL_ALLOW_DEV_KEY", "1")
	os.Exit(m.Run())
}
