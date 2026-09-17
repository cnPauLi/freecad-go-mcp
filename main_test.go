package main

import (
	"strings"
	"testing"
)

func TestValidateHostAcceptsAddressesAndHostnames(t *testing.T) {
	for _, value := range []string{"localhost", "192.168.1.10", "::1", "freecad.example.com", "my-host", "host1"} {
		if err := validateHost(value); err != nil {
			t.Errorf("validateHost(%q) = %v, want no error", value, err)
		}
	}
}

func TestValidateHostRejectsInvalidValues(t *testing.T) {
	values := []string{
		"", " ", "host name", "-bad", "bad-", "a..b", "http://localhost",
		"freecad.example.com:9875", strings.Repeat("a", 64) + ".example.com",
		strings.Repeat("a", 250) + ".com",
	}
	for _, value := range values {
		err := validateHost(value)
		if err == nil {
			t.Errorf("validateHost(%q) = nil, want an error", value)
			continue
		}
		want := "Invalid host: '" + value + "'. Must be a valid IP address or hostname."
		if err.Error() != want {
			t.Errorf("validateHost(%q) = %q, want %q", value, err, want)
		}
	}
}
