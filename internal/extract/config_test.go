package extract

import (
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	data, err := os.ReadFile("../../extract.conf")
	if err != nil {
		panic("failed to load extract.conf: " + err.Error())
	}
	if err := LoadConfigBytes(data); err != nil {
		panic("failed to parse extract.conf: " + err.Error())
	}
	os.Exit(m.Run())
}
