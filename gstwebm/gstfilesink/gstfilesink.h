#ifndef GSTFILESINK_H
#define GSTFILESINK_H

#include <glib.h>
#include <gst/gst.h>
#include <stdint.h>
#include <stdlib.h>

GstElement *gstreamer_recordwebm_create_pipeline(char *pipeline);
void gstreamer_recordwebm_start_pipeline(GstElement *pipeline);
void gstreamer_recordwebm_stop_pipeline(GstElement *pipeline);
void gstreamer_recordwebm_push_buffer_audio(GstElement *pipeline, void *buffer, int len);
void gstreamer_recordwebm_push_buffer_video(GstElement *pipeline, void *buffer, int len);
void gstreamer_recordwebm_start_mainloop(void);

#endif