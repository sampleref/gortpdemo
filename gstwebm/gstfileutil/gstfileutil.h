#ifndef GSTFILEUTIL_H
#define GSTFILEUTIL_H

#include <glib.h>
#include <gst/gst.h>
#include <stdint.h>
#include <stdlib.h>

void gstreamer_create_snap_from_file(char *webmvideofile, char *snapfile);
void gstreamer_get_duration_from_file(char *webmvideofile, char *refid);

extern void goWebmFileDurationCallback(char *refId, int durationSecs);

#endif