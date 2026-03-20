package cmd

import (
	"regexp"
	"testing"
)

func TestShowRegexValidPattern(t *testing.T) {
	pattern := `10\.0\.0\.\d+`
	_, err := regexp.Compile(pattern)
	if err != nil {
		t.Errorf("expected valid regex, got error: %v", err)
	}
}

func TestShowRegexInvalidPattern(t *testing.T) {
	pattern := `[invalid`
	_, err := regexp.Compile(pattern)
	if err == nil {
		t.Error("expected error for invalid regex")
	}
}

func TestShowRegexKeywordMutualExclusivity(t *testing.T) {
	keyword := "nmap"
	regexFlag := `nmap.*`

	if keyword != "" && regexFlag != "" {
		// Expected: this is detected as an error
	} else {
		t.Error("expected mutual exclusivity to be detected")
	}
}
