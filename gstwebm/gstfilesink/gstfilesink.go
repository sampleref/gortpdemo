// Package gstfilesink provides an easy API to create an appsrc pipeline
package gstfilesink

/*
#cgo pkg-config: gstreamer-1.0 gstreamer-app-1.0

#include "gstfilesink.h"

*/
import "C"
import (
	"unsafe"
)

// StartMainLoop starts GLib's main loop
// It needs to be called from the process' main thread
// Because many gstreamer plugins require access to the main thread
// See: https://golang.org/pkg/runtime/#LockOSThread
func StartMainLoop() {
	C.gstreamer_recordwebm_start_mainloop()
}

// Pipeline is a wrapper for a GStreamer Pipeline
type Pipeline struct {
	Pipeline *C.GstElement
}

// CreatePipeline creates a GStreamer Pipeline
func CreatePipeline(filePath string) *Pipeline {
	pipelineStr := "splitmuxsink name=filesink muxer=webmmux location="
	pipelineStr += filePath
	pipelineStr += " appsrc format=time is-live=true do-timestamp=true name=vsrc ! application/x-rtp"
	pipelineStr += ", payload=96, encoding-name=VP8-DRAFT-IETF-01 ! rtpvp8depay ! filesink.video"
	pipelineStr += " appsrc format=time is-live=true do-timestamp=true name=asrc ! application/x-rtp"
	pipelineStr += ", payload=111, encoding-name=OPUS ! rtpopusdepay ! opusdec ! opusenc ! filesink.audio_0"

	pipelineStrUnsafe := C.CString(pipelineStr)
	defer C.free(unsafe.Pointer(pipelineStrUnsafe))
	return &Pipeline{Pipeline: C.gstreamer_recordwebm_create_pipeline(pipelineStrUnsafe)}
}

// Start starts the GStreamer Pipeline
func (p *Pipeline) Start() {
	C.gstreamer_recordwebm_start_pipeline(p.Pipeline)
}

// Stop stops the GStreamer Pipeline
func (p *Pipeline) Stop() {
	C.gstreamer_recordwebm_stop_pipeline(p.Pipeline)
}

// Push pushes a buffer on the appsrc of the GStreamer Pipeline
func (p *Pipeline) PushAudio(buffer []byte) {
	b := C.CBytes(buffer)
	defer C.free(b)
	C.gstreamer_recordwebm_push_buffer_audio(p.Pipeline, b, C.int(len(buffer)))
}

func (p *Pipeline) PushVideo(buffer []byte) {
	b := C.CBytes(buffer)
	defer C.free(b)
	C.gstreamer_recordwebm_push_buffer_video(p.Pipeline, b, C.int(len(buffer)))
}
