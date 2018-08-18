package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"strings"
	"time"

	homedir "github.com/mitchellh/go-homedir"
)

type config struct {
	BaseBgColor string     `yaml:"base_bg_color" json:"base_bg_color"`
	Segments    []*segment `yaml:"segments" json:"segments"`
}

type segment struct {
	BackgroundError   string        `yaml:"background_error" json:"background_error"`
	BackgroundSuccess string        `yaml:"background_success" json:"background_success"`
	ForegroundError   string        `yaml:"foreground_error" json:"foreground_error"`
	ForegroundSuccess string        `yaml:"foreground_success" json:"foreground_success"`
	Command           []string      `yaml:"command" json:"command"`
	Prefix            string        `yaml:"prefix" json:"prefix"`
	Interval          time.Duration `yaml:"interval" json:"interval"`

	Output string
	Err    error
}

func (s segment) cacheKey() string {
	return fmt.Sprintf("%x.json", sha256.Sum256([]byte(strings.Join(s.Command, " "))))
}

func (s segment) storeCache() error {
	if s.Interval == 0 {
		// No interval defined = execute all the time, no caching
		return nil
	}

	p, err := homedir.Expand(path.Join("~", ".cache", "tmux-collector", s.cacheKey()))
	if err != nil {
		return err
	}

	if err = os.MkdirAll(path.Dir(p), 0700); err != nil {
		return err
	}

	fh, err := os.OpenFile(p, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer fh.Close()

	return json.NewEncoder(fh).Encode(s)
}

func (s *segment) loadCache() (bool, error) {
	if s.Interval == 0 {
		// No interval defined = execute all the time, no caching
		return false, nil
	}

	p, err := homedir.Expand(path.Join("~", ".cache", "tmux-collector", s.cacheKey()))
	if err != nil {
		return false, err
	}

	stat, err := os.Stat(p)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	if time.Since(stat.ModTime()) > s.Interval {
		return false, nil
	}

	fh, err := os.Open(p)
	if err != nil {
		return false, err
	}
	defer fh.Close()

	if err = json.NewDecoder(fh).Decode(s); err != nil {
		return false, err
	}

	return true, nil
}
