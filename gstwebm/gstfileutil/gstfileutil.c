#include "gstfileutil.h"

#include "gst/pbutils/pbutils.h"

void gstreamer_create_snap_from_file(char *webmvideofile, char *snapfile){
    GST_INFO("Creating snapshot for: %s", webmvideofile);
    GstElement *pipeline;
    gchar *descr;
    GError *error = NULL;
    GstStateChangeReturn ret;
    descr = g_strdup_printf(
            "filesrc location=%s ! matroskademux ! vp8dec ! videoconvert ! pngenc snapshot=true ! multifilesink location=%s next-file=key-frame",
            webmvideofile, snapfile);
    pipeline = gst_parse_launch(descr, &error);
    if (error != NULL) {
        GST_ERROR("Could not construct pipeline: %s for file %s", error->message, webmvideofile);
        g_error_free(error);
        return;
    }
    ret = gst_element_set_state(pipeline, GST_STATE_PLAYING);
    if (ret == GST_STATE_CHANGE_FAILURE) {
        GST_ERROR("Failed to play file %s", webmvideofile);
        return;
    }
    gst_object_unref(pipeline);
    GST_INFO("Written snapshot for: %s with file %s", webmvideofile, snapfile);
}

/*
 * GST_DISCOVERER TOOLS to measure video file duration - BEGIN
 */
void on_discovered_cb_duration(GstDiscoverer *discoverer, GstDiscovererInfo *info, GError *err, char *refid) {
    GstDiscovererResult result;
    const gchar *uri;
    uri = gst_discoverer_info_get_uri(info);
    result = gst_discoverer_info_get_result(info);
    int duration_secs = 0;
    switch (result) {
        case GST_DISCOVERER_URI_INVALID:
            GST_ERROR ("on_discovered_cb_duration - Invalid URI '%s'\n", uri);
            break;
        case GST_DISCOVERER_ERROR:
            GST_ERROR ("on_discovered_cb_duration - Discoverer error: %s\n", err->message);
            break;
        case GST_DISCOVERER_TIMEOUT:
            GST_ERROR ("on_discovered_cb_duration - Timeout\n");
            break;
        case GST_DISCOVERER_BUSY:
            GST_ERROR ("on_discovered_cb_duration - Busy\n");
            break;
        case GST_DISCOVERER_MISSING_PLUGINS: {
            const GstStructure *s;
            gchar *str;
            s = gst_discoverer_info_get_misc(info);
            str = gst_structure_to_string(s);
            GST_ERROR ("on_discovered_cb_duration - Missing plugins: %s\n", str);
            g_free(str);
            break;
        }
        case GST_DISCOVERER_OK:
            GST_INFO("on_discovered_cb_duration - Discovered '%s'\n", uri);
            GstClockTime duration = gst_discoverer_info_get_duration(info);
            duration_secs = GST_TIME_AS_SECONDS(duration);
            goWebmFileDurationCallback(refid, duration_secs);
            break;
    }
}

/* This function is called when the discoverer has finished examining all the URIs we provided.*/
static void on_finished_cb_duration(GstDiscoverer *discoverer, GMainLoop *discoveryMainLoop) {
    GST_INFO("on_finished_cb_duration - Finished discovering");
    g_main_loop_quit(discoveryMainLoop);
}

/*
* webmvideofile - Any valid path on disk with prefix("file://"), Example: "file://" + <Absolute Path>
* refid - Being async function, callback reference id
*/
void gstreamer_get_duration_from_file(char *webmvideofile, char *refid){
    GError *err = NULL;
    GstDiscoverer *gstDiscoverer;
    GMainLoop *discoveryMainLoop;
    /* Create a GLib Main Loop and set it to run, so we can wait for the signals */
    discoveryMainLoop = g_main_loop_new(NULL, FALSE);
    gstDiscoverer = gst_discoverer_new(5 * GST_SECOND, &err);
    if (!gstDiscoverer) {
        if (err) {
            GST_ERROR ("update_media_file_duration - Error creating discoverer instance: %s", err->message);
            g_error_free(err);
        }
    }
    if (err) {
        GST_ERROR ("update_media_file_duration - Error creating discoverer instance: %s", err->message);
        g_error_free(err);
    }

    /* Connect to the interesting signals */
    g_signal_connect (gstDiscoverer, "discovered", G_CALLBACK(on_discovered_cb_duration), refid);
    g_signal_connect (gstDiscoverer, "finished", G_CALLBACK(on_finished_cb_duration), discoveryMainLoop);
    gst_discoverer_start(gstDiscoverer);
    GST_INFO("update_media_file_duration - Getting info for file: %s", webmvideofile);
    if (!gst_discoverer_discover_uri_async(gstDiscoverer, webmvideofile)) {
        GST_ERROR ("update_media_file_duration - Failed to start discovering URI '%s'\n", webmvideofile);
    }
    g_main_loop_run(discoveryMainLoop);

    /* Stop the discoverer process */
    gst_discoverer_stop(gstDiscoverer);

    /* Free resources */
    g_object_unref(gstDiscoverer);
    g_main_loop_unref(discoveryMainLoop);
}

