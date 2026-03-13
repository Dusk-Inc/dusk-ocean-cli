package main

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
)

func main() {
	configPath := "src/functions/config.go"
	content, err := os.ReadFile(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to read %s: %v\n", configPath, err)
		os.Exit(1)
	}

	pattern := regexp.MustCompile(`var Version = "(\d+)\.(\d+)\.(\d+)"`)
	match := pattern.FindStringSubmatch(string(content))
	if match == nil {
		fmt.Fprintf(os.Stderr, "version pattern not found in %s\n", configPath)
		os.Exit(1)
	}

	major := match[1]
	minor := match[2]
	build, err := strconv.Atoi(match[3])
	if err != nil {
		fmt.Fprintf(os.Stderr, "invalid build number: %s\n", match[3])
		os.Exit(1)
	}

	newVersion := fmt.Sprintf("%s.%s.%d", major, minor, build+1)
	updated := strings.Replace(string(content), match[0], fmt.Sprintf(`var Version = "%s"`, newVersion), 1)

	if err := os.WriteFile(configPath, []byte(updated), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "failed to write %s: %v\n", configPath, err)
		os.Exit(1)
	}

	fmt.Printf("version bumped to %s\n", newVersion)
}
