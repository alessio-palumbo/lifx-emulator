// prepare-responses validates secret-backed release defaults without printing
// their contents. The generated JSON resource is ignored by Git.
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"lifx-emulator/internal/config"
	"lifx-emulator/internal/lan"
)

func prepare(destination, secret string) error {
	if secret == "" {
		return errors.New("LIFX_COMPAT_RESPONSES_JSON is required for a bundled build")
	}
	file, err := config.DecodeResponses([]byte(secret))
	if err != nil {
		return errors.New("invalid bundled response configuration")
	}
	if len(file.Responses) == 0 {
		return errors.New("bundled response configuration must contain at least one rule")
	}
	if err := lan.New(nil, nil).SetResponses(file); err != nil {
		return errors.New("invalid bundled response rules")
	}
	data, err := json.Marshal(file)
	if err != nil {
		return errors.New("could not encode bundled response configuration")
	}
	dir := filepath.Dir(destination)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return errors.New("could not create bundled resource directory")
	}
	temp, err := os.CreateTemp(dir, ".responses-*")
	if err != nil {
		return errors.New("could not create bundled resource")
	}
	defer os.Remove(temp.Name())
	if _, err := temp.Write(data); err != nil {
		temp.Close()
		return errors.New("could not write bundled resource")
	}
	if err := temp.Close(); err != nil {
		return errors.New("could not close bundled resource")
	}
	if err := os.Rename(temp.Name(), destination); err != nil {
		return errors.New("could not install bundled resource")
	}
	return nil
}

func main() {
	if err := prepare("internal/config/bundled/compat.local.json", os.Getenv("LIFX_COMPAT_RESPONSES_JSON")); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println("Bundled response configuration validated and prepared")
}
