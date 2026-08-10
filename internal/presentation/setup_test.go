package presentation

import (
	"os"
	"testing"

	"github.com/fatih/color"
)

func TestMain(m *testing.M) {
	color.NoColor = false
	os.Exit(m.Run())
}
