package astiunderstanding

import "context"

// Constants
const (
	name = "Understanding"
)

// Preparer represents an object capable of preparing training data
type Preparer interface {
	Prepare(ctx context.Context, srcDir, dstDir string) error
	SetFuncs(fnError func(id string, err error), fnSkip, fnSuccess func(id string))
}

// SilenceDetector represents an object capable of detecting valid samples between silences
type SilenceDetector interface {
	Add(samples []int32, sampleRate int, silenceMaxAudioLevel float64) (validSamples [][]int32)
	Reset()
}

// SpeechParser represents an object capable of parsing speech and returning the corresponding text
type SpeechParser interface {
	SpeechToText(samples []int32, sampleRate, significantBits int) (string, error)
}

// Websocket event names
const (
	websocketEventNameAnalysis           = "analysis"
	websocketEventNamePrepareCancel      = "prepare.cancel"
	websocketEventNamePrepareDone        = "prepare.done"
	websocketEventNamePrepareItemError   = "prepare.item.error"
	websocketEventNamePrepareItemSkip    = "prepare.item.skip"
	websocketEventNamePrepareItemSuccess = "prepare.item.success"
	websocketEventNamePrepareStart       = "prepare.start"
	websocketEventNameSamples            = "samples"
	websocketEventNameSamplesStored      = "samples.stored"
)
