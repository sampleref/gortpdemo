package gstwebm

import (
	"fmt"
	"github.com/pion/rtp"
	"github.com/pion/rtp/codecs"
	"github.com/pion/webrtc/v3/pkg/media/samplebuilder"
	gstfs "github.com/sampleref/gortpdemo/gstwebm/gstfilesink"
	"runtime"
)

var (
	pipeline        *gstfs.Pipeline
	pipelinePlaying = false

	audioBuilder = samplebuilder.New(10, &codecs.OpusPacket{}, 48000)
	videoBuilder = samplebuilder.New(10, &codecs.VP8Packet{}, 90000)
)

func gstreamerRecordWebmCreateStartPipeline(fileName string) {
	pipeline = gstfs.CreatePipeline(fileName)
	pipeline.Start()
	pipelinePlaying = true

	fmt.Println("Starting main loop for recording pipeline")
	// Use this goroutine (which has been runtime.LockOSThread'd to he the main thread) to run the Glib loop that Gstreamer requires
	gstfs.StartMainLoop()
	fmt.Println("Started main loop for recording pipeline")
}

func gstreamerRecordWebmCreateStopPipeline() {
	pipelinePlaying = false
	pipeline.Stop()
}

func PushAudioRTP(rtp *rtp.Packet) {
	buf := make([]byte, 1400)
	i, readErr := rtp.MarshalTo(buf)
	if readErr != nil {
		panic(readErr)
	}
	if pipelinePlaying && buf != nil {
		pipeline.PushAudio(buf[:i])
	}
}

func PushVideoRTP(rtp *rtp.Packet) {
	buf := make([]byte, 1400)
	i, readErr := rtp.MarshalTo(buf)
	if readErr != nil {
		panic(readErr)
	}
	if pipelinePlaying && buf != nil {
		pipeline.PushVideo(buf[:i])
	}
}

func init() {
	// This example uses Gstreamer's autovideosink element to display the received video
	// This element, along with some others, sometimes require that the process' main thread is used
	runtime.LockOSThread()
}

func Start(fileName string) {
	// Start a new thread to do the actual work for this application
	go gstreamerRecordWebmCreateStartPipeline(fileName)
}

func Stop() {
	go gstreamerRecordWebmCreateStopPipeline()
}
