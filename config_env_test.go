package main

import (
	"os"
	"testing"
)

func TestEnvironmentOverridesFileConfiguration(t *testing.T) {
	base := serverConfig{Port: 8080, CityName: "南宁", CityID: "101300101", TimezoneOffset: 8}
	t.Setenv("CITY_NAME", " 上海 ")
	t.Setenv("CITY_ID", "")
	t.Setenv("TIMEZONE_OFFSET", "0")
	got := applyEnvironment(base)
	if got.CityName != "上海" || got.CityID != "" || got.TimezoneOffset != 0 || got.Port != 8080 {
		t.Fatalf("unexpected settings: %+v", got)
	}
}
func TestInvalidEnvironmentPreservesFileConfiguration(t *testing.T) {
	base := serverConfig{Port: 8080, CityName: "南宁", CityID: "101300101", TimezoneOffset: 8}
	t.Setenv("CITY_NAME", "")
	t.Setenv("CITY_ID", "bad")
	t.Setenv("TIMEZONE_OFFSET", "15")
	if got := applyEnvironment(base); got != base {
		t.Fatalf("unexpected settings: %+v", got)
	}
}

func TestLoadConfigUsesEnvironmentWithoutFile(t *testing.T) {
	t.Chdir(t.TempDir())
	t.Setenv("CITY_NAME", "北京")
	t.Setenv("CITY_ID", "101010100")
	t.Setenv("TIMEZONE_OFFSET", "0")
	got := loadConfig()
	if got.CityName != "北京" || got.CityID != "101010100" || got.TimezoneOffset != 0 {
		t.Fatalf("unexpected settings: %+v", got)
	}
}
func TestLoadConfigEnvironmentOverridesJSON(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.WriteFile("config.json", []byte(`{"city_name":"南宁","timezone_offset":8,"port":8080}`), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CITY_NAME", "上海")
	t.Setenv("CITY_ID", "")
	t.Setenv("TIMEZONE_OFFSET", "3")
	got := loadConfig()
	if got.CityName != "上海" || got.TimezoneOffset != 3 {
		t.Fatalf("unexpected settings: %+v", got)
	}
}
