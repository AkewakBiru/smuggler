package smuggler_test

import (
	"smuggler/smuggler"
	"testing"
)

func TestWriteBasic(t *testing.T) {
	smuggler.WriteBasic()
}

func TestWriteDouble(t *testing.T) {
	smuggler.WriteDouble()
}

func TestWriteExhaustive(t *testing.T) {
	smuggler.WriteExhaustive()
}
