// Package provides API for gst utils
package gstfileutil

/*
#cgo pkg-config: gstreamer-1.0 gstreamer-pbutils-1.0

#include "gstfileutil.h"

*/
import "C"
import (
	"fmt"
	"unsafe"
)

// CreateSnapFromWebmFile creates snap of webm file
func CreateSnapFromWebmFile(filePath string, snapPath string) {
	filePathStrUnsafe := C.CString(filePath)
	snapPathStrUnsafe := C.CString(snapPath)
	defer C.free(unsafe.Pointer(filePathStrUnsafe))
	defer C.free(unsafe.Pointer(snapPathStrUnsafe))
	C.gstreamer_create_snap_from_file(filePathStrUnsafe, snapPathStrUnsafe)
}

func RequestDurationFromWebmFile(filePath string, callbackRefId string) {
	filePathStrUnsafe := C.CString(filePath)
	callbackRefIdStrUnsafe := C.CString(callbackRefId)
	defer C.free(unsafe.Pointer(filePathStrUnsafe))
	defer C.free(unsafe.Pointer(callbackRefIdStrUnsafe))
	C.gstreamer_get_duration_from_file(filePathStrUnsafe, callbackRefIdStrUnsafe)
}

//export goWebmFileDurationCallback
func goWebmFileDurationCallback(refId *C.char, durationSecs C.int) {
	fmt.Printf("Received duration for refId %s as %d \n", C.GoString(refId), int(durationSecs))
}
