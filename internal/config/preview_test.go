package config

import "testing"

func TestPreviewConverterConfigurationReachesApplication(t *testing.T) {
	t.Setenv("JUDEX_ENV", "test")
	t.Setenv("JUDEX_DATABASE_URL", "postgres://test/test")
	t.Setenv("JUDEX_MATERIAL_CONVERTER_URL", "http://converter:9081")
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if c.MaterialConverterURL != "http://converter:9081" {
		t.Fatal("converter endpoint discarded")
	}
}
