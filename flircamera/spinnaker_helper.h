#ifndef SPINNAKER_HELPER_H
#define SPINNAKER_HELPER_H

#include <stddef.h>

// Opaque camera context.
typedef struct camera_context camera_context;

// Create a camera context: initialize the Spinnaker system, find a camera
// (by serial_number if non-NULL/empty, otherwise first camera), and initialize it.
// Returns 0 on success, negative on error.
int camera_context_create(const char *serial_number, camera_context **out_ctx);

// Configure exposure and gain. auto_exposure/auto_gain: 1=auto, 0=manual.
// When manual, exposure_us/gain_db are applied (ignored if <= 0).
// Returns 0 on success.
int camera_configure(camera_context *ctx, double exposure_us, double gain_db,
                     int auto_exposure, int auto_gain);

// Set acquisition mode to Continuous and begin acquisition.
// Returns 0 on success.
int camera_start(camera_context *ctx);

// Grab the next frame, convert to RGB8, and return a pointer to the RGB data.
// The data pointer is valid until the next call to camera_grab_rgb or camera_destroy.
// Returns 0 on success.
int camera_grab_rgb(camera_context *ctx, unsigned char **data,
                    size_t *width, size_t *height, size_t *size);

// End acquisition.
// Returns 0 on success.
int camera_stop(camera_context *ctx);

// Tear down everything: stop acquisition, deinit camera, release all handles.
void camera_destroy(camera_context *ctx);

#endif // SPINNAKER_HELPER_H
