package gstwebm

import "github.com/sampleref/gortpdemo/gstwebm/gstfileutil"

func CreateSnapForFile(filePath string, snapPath string) {
	gstfileutil.CreateSnapFromWebmFile(filePath, snapPath)
}

func RequestDurationForFile(filePath string, callbackRefId string) {
	gstfileutil.RequestDurationFromWebmFile(filePath, callbackRefId)
}
