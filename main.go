package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"

	"github.com/Luzifer/rconfig"
	log "github.com/sirupsen/logrus"
	yaml "gopkg.in/yaml.v2"
)

var (
	cfg = struct {
		Config         string `flag:"config,c" default:"config.yml" description:"Configuration file"`
		LogLevel       string `flag:"log-level" default:"info" description:"Log level (debug, info, warn, error, fatal)"`
		VersionAndExit bool   `flag:"version" default:"false" description:"Prints current version and exits"`
	}{}

	version = "dev"
)

func init() {
	if err := rconfig.ParseAndValidate(&cfg); err != nil {
		log.Fatalf("Unable to parse commandline options: %s", err)
	}

	if cfg.VersionAndExit {
		fmt.Printf("tmux-collector %s\n", version)
		os.Exit(0)
	}

	if l, err := log.ParseLevel(cfg.LogLevel); err != nil {
		log.WithError(err).Fatal("Unable to parse log level")
	} else {
		log.SetLevel(l)
	}
}

func main() {
	conf, err := loadConfig()
	if err != nil {
		log.WithError(err).Fatal("Unable to load config")
	}

	wg := &sync.WaitGroup{}
	wg.Add(len(conf.Segments))
	for _, seg := range conf.Segments {
		go executeSegment(seg, wg)
	}
	wg.Wait()

	lastBgColor := conf.BaseBgColor
	output := []string{}

	for _, seg := range conf.Segments {
		bg := seg.BackgroundSuccess
		fg := seg.ForegroundSuccess

		if seg.Err != nil {
			if seg.BackgroundError != "" {
				bg = seg.BackgroundError
			}
			if seg.ForegroundError != "" {
				fg = seg.ForegroundError
			}

			if strings.TrimSpace(seg.Output) == "" {
				seg.Output = seg.Err.Error()
			}
		}

		if strings.TrimSpace(seg.Output) == "" {
			continue
		}

		if seg.Prefix != "" {
			seg.Output = strings.Join([]string{seg.Prefix, seg.Output}, " ")
		}

		if lastBgColor != bg {
			output = append(output, fmt.Sprintf(" #[fg=%s,bg=%s]#[fg=%s,bg=%s] ",
				bg, lastBgColor, fg, bg,
			))
		} else {
			output = append(output, fmt.Sprintf("#[fg=%s,bg=%s] ",
				fg, bg,
			))
		}
		output = append(output, seg.Output)
		lastBgColor = bg
	}

	fmt.Print(strings.Join(output, ""))
}

func executeSegment(seg *segment, wg *sync.WaitGroup) {
	defer wg.Done()

	loaded, err := seg.loadCache()
	if err != nil {
		log.WithError(err).Error("Unable to load cache")
	}
	if loaded {
		log.WithField("command", strings.Join(seg.Command, " ")).Debug("Loaded from cache")
		return
	}

	buf := new(bytes.Buffer)

	cmd := exec.Command(seg.Command[0], seg.Command[1:]...)
	cmd.Stdout = buf

	seg.Err = cmd.Run()
	seg.Output = strings.Split(buf.String(), "\n")[0]
	log.WithField("command", strings.Join(seg.Command, " ")).Debug("Freshly loaded")

	if err = seg.storeCache(); err != nil {
		log.WithError(err).Error("Unable to store cache")
	}
}

func loadConfig() (*config, error) {
	f, err := os.Open(cfg.Config)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	out := &config{}
	return out, yaml.NewDecoder(f).Decode(out)
}
