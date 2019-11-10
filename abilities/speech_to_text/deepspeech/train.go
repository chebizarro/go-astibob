package deepspeech

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/asticode/go-astibob/abilities/speech_to_text"
	"github.com/asticode/go-astilog"
	astiexec "github.com/asticode/go-astitools/exec"
	"github.com/pkg/errors"
)

// Regexps
var (
	regexpEpoch = regexp.MustCompile("^I Finished training epoch ([\\d]+)")
)

func (d *DeepSpeech) train(ctx context.Context, speeches []speech_to_text.SpeechFile, progressFunc func(speech_to_text.Progress), p *speech_to_text.Progress) (err error) {
	// Update progress
	p.CurrentStep = trainingStep
	p.Progress = 0
	progressFunc(*p)

	// Create name
	name := "python3"
	if d.o.PythonBinaryPath != "" {
		name = d.o.PythonBinaryPath
	}

	// Create args
	args := d.o.TrainingArgs
	if args == nil {
		args = make(map[string]string)
	}

	// Add mandatory args
	args["train_files"] = filepath.Join(d.o.PrepareDirPath, "train.csv")
	args["dev_files"] = filepath.Join(d.o.PrepareDirPath, "dev.csv")
	args["test_files"] = filepath.Join(d.o.PrepareDirPath, "test.csv")
	args["alphabet_config_path"] = d.o.AlphabetPath
	args["lm_binary_path"] = d.o.LMPath
	args["lm_trie_path"] = d.o.TriePath
	args["audio_sample_rate"] = strconv.Itoa(deepSpeechSampleRate)
	args["noshow_progressbar"] = ""

	// Default args
	if _, ok := args["epochs"]; !ok {
		args["epochs"] = "75"
	}

	// Get number of epochs
	var numEpochs int
	if numEpochs, err = strconv.Atoi(args["epochs"]); err != nil {
		err = errors.Wrapf(err, "deepspeech: atoi of %s failed", args["epochs"])
		return
	}

	// Create command
	cmd := exec.CommandContext(ctx, name, append([]string{"-u", d.o.ClientPath}, argsToSlice(args)...)...)

	// Intercept stderr
	var stderr [][]byte
	cmd.Stderr = astiexec.NewStdWriter(func(i []byte) {
		// Log
		astilog.Debugf("deepspeech: stderr: %s", i)

		// Append
		stderr = append(stderr, i)
	})

	// Intercept stdout
	var epoch int
	var errStdOut error
	cmd.Stdout = astiexec.NewStdWriter(func(i []byte) {
		// Log
		astilog.Debugf("deepspeech: stdout: %s", i)

		// Parse epoch
		if ms := regexpEpoch.FindStringSubmatch(string(i)); len(ms) >= 2 {
			// Convert to int
			if epoch, errStdOut = strconv.Atoi(ms[1]); err != nil {
				astilog.Error(errors.Wrapf(errStdOut, "deepspeech: atoi of %s failed", ms[1]))
			}

			// Update progress
			// We can't have the progress be 100 when epoch == numEpoch since at that time the binary is still running
			// Epoch starts at 0
			p.Progress = 1 + (float64(epoch) / float64(numEpochs) * 98)
			progressFunc(*p)
		}
	})

	// Create the illusion we're doing something :D
	p.Progress = 1
	progressFunc(*p)

	// Run
	astilog.Debugf("deepspeech: running %s", strings.Join(cmd.Args, " "))
	if err = cmd.Run(); err != nil {
		var m string
		if len(stderr) > 0 {
			m = fmt.Sprintf(" with stderr:\n\n%s\n\n", bytes.Join(stderr, []byte("\n")))
		}
		err = errors.Wrapf(err, "deepspeech: running %s failed%s", strings.Join(cmd.Args, " "), m)
		return
	}

	// Update progress
	p.Progress = 100
	progressFunc(*p)
	return
}

func (d *DeepSpeech) trainHashPath() string {
	return filepath.Join(filepath.Dir(d.o.ModelPath), "hash")
}

func argsToSlice(args map[string]string) (o []string) {
	for k, v := range args {
		o = append(o, "--"+k)
		if v != "" {
			o = append(o, v)
		}
	}
	return
}
