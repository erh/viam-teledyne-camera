#include "spinnaker_helper.h"
#include "SpinnakerC.h"

#include <stdlib.h>
#include <string.h>
#include <stdio.h>

struct camera_context {
    spinSystem system;
    spinCameraList camera_list;
    spinCamera camera;
    spinNodeMapHandle node_map;
    spinImage converted_image;
    spinImageProcessor image_processor;
    int acquiring;
};

// Helper: set an enumeration node by entry name.
static spinError set_enum_node(spinNodeMapHandle node_map,
                               const char *node_name,
                               const char *entry_name) {
    spinNodeHandle hNode = NULL;
    spinError err = spinNodeMapGetNode(node_map, node_name, &hNode);
    if (err != SPINNAKER_ERR_SUCCESS) return err;

    spinNodeHandle hEntry = NULL;
    err = spinEnumerationGetEntryByName(hNode, entry_name, &hEntry);
    if (err != SPINNAKER_ERR_SUCCESS) return err;

    int64_t value = 0;
    err = spinEnumerationEntryGetIntValue(hEntry, &value);
    if (err != SPINNAKER_ERR_SUCCESS) return err;

    return spinEnumerationSetIntValue(hNode, value);
}

// Helper: set a float node value.
static spinError set_float_node(spinNodeMapHandle node_map,
                                const char *node_name, double value) {
    spinNodeHandle hNode = NULL;
    spinError err = spinNodeMapGetNode(node_map, node_name, &hNode);
    if (err != SPINNAKER_ERR_SUCCESS) return err;
    return spinFloatSetValue(hNode, value);
}

int camera_context_create(const char *serial_number,
                          camera_context **out_ctx) {
    camera_context *ctx = (camera_context *)calloc(1, sizeof(camera_context));
    if (!ctx) return -1;

    spinError err;

    // Get system instance.
    err = spinSystemGetInstance(&ctx->system);
    if (err != SPINNAKER_ERR_SUCCESS) goto fail_free;

    // Create and populate camera list.
    err = spinCameraListCreateEmpty(&ctx->camera_list);
    if (err != SPINNAKER_ERR_SUCCESS) goto fail_system;

    err = spinSystemGetCameras(ctx->system, ctx->camera_list);
    if (err != SPINNAKER_ERR_SUCCESS) goto fail_camlist;

    size_t num_cameras = 0;
    spinCameraListGetSize(ctx->camera_list, &num_cameras);
    if (num_cameras == 0) {
        err = -2; // no cameras found
        goto fail_camlist;
    }

    // Find camera by serial or take first.
    if (serial_number && serial_number[0] != '\0') {
        int found = 0;
        for (size_t i = 0; i < num_cameras; i++) {
            spinCamera hCam = NULL;
            err = spinCameraListGet(ctx->camera_list, i, &hCam);
            if (err != SPINNAKER_ERR_SUCCESS) continue;

            spinNodeMapHandle hTLNodeMap = NULL;
            err = spinCameraGetTLDeviceNodeMap(hCam, &hTLNodeMap);
            if (err != SPINNAKER_ERR_SUCCESS) {
                spinCameraRelease(hCam);
                continue;
            }

            spinNodeHandle hSerial = NULL;
            err = spinNodeMapGetNode(hTLNodeMap, "DeviceSerialNumber",
                                     &hSerial);
            if (err != SPINNAKER_ERR_SUCCESS) {
                spinCameraRelease(hCam);
                continue;
            }

            char buf[256];
            size_t buf_len = sizeof(buf);
            err = spinStringGetValue(hSerial, buf, &buf_len);
            if (err == SPINNAKER_ERR_SUCCESS &&
                strcmp(buf, serial_number) == 0) {
                ctx->camera = hCam;
                found = 1;
                break;
            }
            spinCameraRelease(hCam);
        }
        if (!found) {
            err = -3; // camera with serial not found
            goto fail_camlist;
        }
    } else {
        err = spinCameraListGet(ctx->camera_list, 0, &ctx->camera);
        if (err != SPINNAKER_ERR_SUCCESS) goto fail_camlist;
    }

    // Initialize camera.
    err = spinCameraInit(ctx->camera);
    if (err != SPINNAKER_ERR_SUCCESS) goto fail_camera;

    // Get node map.
    err = spinCameraGetNodeMap(ctx->camera, &ctx->node_map);
    if (err != SPINNAKER_ERR_SUCCESS) goto fail_deinit;

    // Pre-allocate a reusable destination image for format conversion.
    err = spinImageCreateEmpty(&ctx->converted_image);
    if (err != SPINNAKER_ERR_SUCCESS) goto fail_deinit;

    // Create image processor for pixel format conversion.
    err = spinImageProcessorCreate(&ctx->image_processor);
    if (err != SPINNAKER_ERR_SUCCESS) goto fail_image;

    // Use high-quality linear demosaicing for Bayer → RGB conversion.
    spinImageProcessorSetColorProcessing(ctx->image_processor,
        SPINNAKER_COLOR_PROCESSING_ALGORITHM_HQ_LINEAR);

    *out_ctx = ctx;
    return 0;

fail_image:
    spinImageDestroy(ctx->converted_image);
fail_deinit:
    spinCameraDeInit(ctx->camera);
fail_camera:
    spinCameraRelease(ctx->camera);
fail_camlist:
    spinCameraListClear(ctx->camera_list);
    spinCameraListDestroy(ctx->camera_list);
fail_system:
    spinSystemReleaseInstance(ctx->system);
fail_free:
    free(ctx);
    return (int)err;
}

int camera_configure(camera_context *ctx, double exposure_us, double gain_db,
                     int auto_exposure, int auto_gain) {
    // Configure exposure.
    if (auto_exposure) {
        set_enum_node(ctx->node_map, "ExposureAuto", "Continuous");
    } else {
        set_enum_node(ctx->node_map, "ExposureAuto", "Off");
        if (exposure_us > 0) {
            set_float_node(ctx->node_map, "ExposureTime", exposure_us);
        }
    }

    // Configure gain.
    if (auto_gain) {
        set_enum_node(ctx->node_map, "GainAuto", "Continuous");
    } else {
        set_enum_node(ctx->node_map, "GainAuto", "Off");
        if (gain_db > 0) {
            set_float_node(ctx->node_map, "Gain", gain_db);
        }
    }

    // Enable continuous auto white balance.
    set_enum_node(ctx->node_map, "BalanceWhiteAuto", "Continuous");

    return 0;
}

int camera_start(camera_context *ctx) {
    spinError err;

    err = set_enum_node(ctx->node_map, "AcquisitionMode", "Continuous");
    if (err != SPINNAKER_ERR_SUCCESS) return (int)err;

    err = spinCameraBeginAcquisition(ctx->camera);
    if (err != SPINNAKER_ERR_SUCCESS) return (int)err;

    ctx->acquiring = 1;
    return 0;
}

int camera_grab_rgb(camera_context *ctx, unsigned char **data,
                    size_t *width, size_t *height, size_t *size) {
    if (!ctx->acquiring) return -4;

    spinImage hImage = NULL;
    spinError err;

    // Grab with 5-second timeout.
    err = spinCameraGetNextImageEx(ctx->camera, 5000, &hImage);
    if (err != SPINNAKER_ERR_SUCCESS) return (int)err;

    // Check for incomplete image.
    bool8_t is_incomplete = False;
    spinImageIsIncomplete(hImage, &is_incomplete);
    if (is_incomplete) {
        spinImageRelease(hImage);
        return -5;
    }

    // Convert to RGB8 using image processor.
    err = spinImageProcessorConvert(ctx->image_processor, hImage,
                                    ctx->converted_image, PixelFormat_RGB8);
    spinImageRelease(hImage);
    if (err != SPINNAKER_ERR_SUCCESS) return (int)err;

    // Return converted image properties.
    spinImageGetWidth(ctx->converted_image, width);
    spinImageGetHeight(ctx->converted_image, height);
    spinImageGetBufferSize(ctx->converted_image, size);

    void *pData = NULL;
    spinImageGetData(ctx->converted_image, &pData);
    *data = (unsigned char *)pData;

    return 0;
}

int camera_stop(camera_context *ctx) {
    if (ctx->acquiring) {
        spinCameraEndAcquisition(ctx->camera);
        ctx->acquiring = 0;
    }
    return 0;
}

void camera_destroy(camera_context *ctx) {
    if (!ctx) return;

    if (ctx->acquiring) {
        spinCameraEndAcquisition(ctx->camera);
    }

    if (ctx->image_processor) {
        spinImageProcessorDestroy(ctx->image_processor);
    }

    if (ctx->converted_image) {
        spinImageDestroy(ctx->converted_image);
    }

    if (ctx->camera) {
        spinCameraDeInit(ctx->camera);
        spinCameraRelease(ctx->camera);
    }

    if (ctx->camera_list) {
        spinCameraListClear(ctx->camera_list);
        spinCameraListDestroy(ctx->camera_list);
    }

    if (ctx->system) {
        spinSystemReleaseInstance(ctx->system);
    }

    free(ctx);
}
