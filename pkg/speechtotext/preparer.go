package astispeechtotext

import (
	"bytes"
	"context"
	"encoding/csv"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/asticode/go-astitools/audio"
	"github.com/cryptix/wav"
	"github.com/pkg/errors"
)

// Vars
var (
	regexpNonLetters = regexp.MustCompile("[^\\w\\s'-]*")
)

// Preparer represents an object capable of preparing training data
type Preparer struct {
	fnError   func(id string, err error)
	fnSkip    func(id string)
	fnSuccess func(id string)
}

// NewPreparer creates a new Preparer
func NewPreparer() *Preparer {
	return &Preparer{}
}

// SetFuncs implements the astiunderstanding.Preparer interface
func (p *Preparer) SetFuncs(fnError func(id string, err error), fnSkip, fnSuccess func(id string)) {
	p.fnError = fnError
	p.fnSkip = fnSkip
	p.fnSuccess = fnSuccess
}

// Prepare implements the astiunderstanding.Preparer interface
func (p *Preparer) Prepare(ctx context.Context, srcDir, dstDir string) (err error) {
	// Stat csv
	csvPath := filepath.Join(dstDir, "index.csv")
	_, errStat := os.Stat(csvPath)
	if errStat != nil && !os.IsNotExist(errStat) {
		err = errors.Wrapf(err, "astispeechtotext: stating %s failed", csvPath)
		return
	}

	// Create csv dir
	dirPath := filepath.Dir(csvPath)
	if err = os.MkdirAll(dirPath, 0755); err != nil {
		err = errors.Wrapf(err, "astispeechtotext: mkdirall %s failed", dirPath)
		return
	}

	// Open csv
	csvFile, err := os.OpenFile(csvPath, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0755)
	if err != nil {
		err = errors.Wrapf(err, "astispeechtotext: opening %s failed", csvPath)
		return
	}
	defer csvFile.Close()

	// Create csv writer
	w := csv.NewWriter(csvFile)
	defer w.Flush()

	// Check whether csv existed
	indexedWavFilenames := make(map[string]bool)
	if os.IsNotExist(errStat) {
		// Write header
		if err = w.Write([]string{
			"wav_filename",
			"wav_filesize",
			"transcript",
		}); err != nil {
			err = errors.Wrap(err, "astispeechtotext: writing csv header failed")
			return
		}
		w.Flush()
	} else {
		// Create csv reader
		r := csv.NewReader(csvFile)
		r.FieldsPerRecord = 3

		// Read all
		records, err := r.ReadAll()
		if err != nil {
			err = errors.Wrap(err, "astispeechtotext: reading all csv failed")
			return
		}

		// Index wav filenames
		for _, record := range records {
			indexedWavFilenames[record[0]] = true
		}
	}

	// Walk in input path
	if err = filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		// Check context
		if ctx.Err() != nil {
			return ctx.Err()
		}

		// Process error
		if err != nil {
			return err
		}

		// Only process wav files
		if info.IsDir() || !strings.HasSuffix(path, ".wav") {
			return nil
		}

		// Get id
		id := strings.TrimSuffix(strings.TrimPrefix(path, srcDir), ".wav")

		// ID has already been processed
		wavOutputPath := filepath.Join(dstDir, id+".wav")
		if _, ok := indexedWavFilenames[wavOutputPath]; ok {
			p.fnSkip(id)
			return nil
		}

		// Retrieve transcript
		var transcript []byte
		txtPath := filepath.Join(srcDir, id+".txt")
		if transcript, err = p.retrieveTranscript(txtPath); err != nil {
			p.fnError(id, errors.Wrapf(err, "astispeechtotext: retrieving transcript from %s failed", txtPath))
			return nil
		}

		// Convert wav file
		wavInputPath := filepath.Join(srcDir, id+".wav")
		if err = p.convertWavFile(wavInputPath, wavOutputPath); err != nil {
			p.fnError(id, errors.Wrapf(err, "astispeechtotext: converting wav file from %s to %s failed", wavInputPath, wavOutputPath))
			return nil
		}

		// Append to csv
		if err = p.appendToCSV(w, wavOutputPath, string(transcript)); err != nil {
			p.fnError(id, errors.Wrapf(err, "astispeechtotext: appending %s with transcript %s to csv failed", wavOutputPath, transcript))
			return nil
		}

		// Success
		p.fnSuccess(id)
		return nil
	}); err != nil {
		err = errors.Wrapf(err, "astispeechtotext: walking through %s failed", srcDir)
		return
	}
	return
}

func (p *Preparer) retrieveTranscript(txtPath string) (transcript []byte, err error) {
	// Read src
	if transcript, err = ioutil.ReadFile(txtPath); err != nil {
		err = errors.Wrapf(err, "astispeechtotext: reading %s failed", txtPath)
	}

	// To lower
	transcript = bytes.ToLower(transcript)

	// Remove non letters
	transcript = regexpNonLetters.ReplaceAll(transcript, []byte(""))
	return
}

func (p *Preparer) convertWavFile(src, dst string) (err error) {
	// Stat src
	var fi os.FileInfo
	if fi, err = os.Stat(src); err != nil {
		return errors.Wrapf(err, "astispeechtotext: stating %s failed", src)
	}

	// Open src
	var srcFile *os.File
	if srcFile, err = os.Open(src); err != nil {
		return errors.Wrapf(err, "astispeechtotext: opening %s failed", src)
	}
	defer srcFile.Close()

	// Create wav reader
	var r *wav.Reader
	if r, err = wav.NewReader(srcFile, fi.Size()); err != nil {
		return errors.Wrap(err, "astispeechtotext: creating wav reader failed")
	}

	// Get samples
	var samples []int32
	var sample int32
	for {
		// Read sample
		if sample, err = r.ReadSample(); err != nil {
			if err != io.EOF {
				return errors.Wrap(err, "astispeechtotext: reading wav sample failed")
			}
			break
		}

		// Append sample
		samples = append(samples, sample)
	}

	// Create dst dir
	dstDir := filepath.Dir(dst)
	if err = os.MkdirAll(dstDir, 0755); err != nil {
		return errors.Wrapf(err, "astispeechtotext: mkdirall %s failed", dstDir)
	}

	// Create dst file
	var dstFile *os.File
	if dstFile, err = os.Create(dst); err != nil {
		return errors.Wrapf(err, "astispeechtotext: creating %s failed", dst)
	}
	defer dstFile.Close()

	// Create wav file
	wavFile := wav.File{
		Channels:        1,
		SampleRate:      16000,
		SignificantBits: 16,
	}

	// Create wav writer
	var w *wav.Writer
	if w, err = wavFile.NewWriter(dstFile); err != nil {
		return errors.Wrap(err, "astispeechtotext: creating wav writer failed")
	}
	defer w.Close()

	// Convert sample rate
	if samples, err = astiaudio.ConvertSampleRate(samples, int(r.GetFile().SampleRate), int(wavFile.SampleRate)); err != nil {
		return errors.Wrap(err, "astispeechtotext: converting sample rate failed")
	}

	// Loop through samples
	for _, sample := range samples {
		// Convert bit depth
		if sample, err = astiaudio.ConvertBitDepth(sample, int(r.GetFile().SignificantBits), int(wavFile.SignificantBits)); err != nil {
			return errors.Wrap(err, "astispeechtotext: converting bit depth failed")
		}

		// Write
		if err = w.WriteSample([]byte{byte(sample & 0xff), byte(sample >> 8 & 0xff)}); err != nil {
			return errors.Wrap(err, "astispeechtotext: writing wav sample failed")
		}
	}
	return
}

func (p *Preparer) appendToCSV(w *csv.Writer, wavPath, transcript string) (err error) {
	// Stat wav
	var fi os.FileInfo
	if fi, err = os.Stat(wavPath); err != nil {
		return errors.Wrapf(err, "astispeechtotext: stating %s failed", wavPath)
	}

	// Write
	if err = w.Write([]string{
		wavPath,
		strconv.Itoa(int(fi.Size())),
		transcript,
	}); err != nil {
		return errors.Wrap(err, "astispeechtotext: writing csv data failed")
	}
	w.Flush()
	return
}
