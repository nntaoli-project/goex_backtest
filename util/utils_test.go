package util

import (
	"testing"
)

func TestLoadTomlConfig(t *testing.T) {
	t.Log(LoadTomlConfig("binance.com_sim.toml"))
}
