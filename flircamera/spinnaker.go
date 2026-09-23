package flircamera

/*
#cgo CFLAGS: -I/opt/spinnaker/include/spinc -I/opt/spinnaker/include
#cgo LDFLAGS: -L/opt/spinnaker/lib -lSpinnaker_C
#include "spinnaker_helper.h"
#include <stdlib.h>
*/
import "C"
import (
	"fmt"
	"unsafe"
)

// spinnakerCamera wraps the C camera context.
type spinnakerCamera struct {
	ctx *C.camera_context
}

// newSpinnakerCamera opens and initializes a camera.
// If serialNumber is empty, the first available camera is used.
func newSpinnakerCamera(serialNumber string) (*spinnakerCamera, error) {
	var ctx *C.camera_context
	var cSerial *C.char
	if serialNumber != "" {
		cSerial = C.CString(serialNumber)
		defer C.free(unsafe.Pointer(cSerial))
	}

	ret := C.camera_context_create(cSerial, &ctx)
	if ret != 0 {
		return nil, fmt.Errorf("camera_context_create failed (code %d)", int(ret))
	}
	return &spinnakerCamera{ctx: ctx}, nil
}

// configure sets exposure and gain parameters.
func (sc *spinnakerCamera) configure(exposureUs, gainDb float64, autoExposure, autoGain bool) error {
	autoExp := C.int(0)
	if autoExposure {
		autoExp = 1
	}
	autoG := C.int(0)
	if autoGain {
		autoG = 1
	}

	ret := C.camera_configure(sc.ctx, C.double(exposureUs), C.double(gainDb), autoExp, autoG)
	if ret != 0 {
		return fmt.Errorf("camera_configure failed (code %d)", int(ret))
	}
	return nil
}

// start begins continuous acquisition.
func (sc *spinnakerCamera) start() error {
	ret := C.camera_start(sc.ctx)
	if ret != 0 {
		return fmt.Errorf("camera_start failed (code %d)", int(ret))
	}
	return nil
}

// grabRGB grabs the next frame as RGB8 data.
// Returns a copy of the pixel data (safe to keep), width, and height.
func (sc *spinnakerCamera) grabRGB() ([]byte, int, int, error) {
	var data *C.uchar
	var width, height, size C.size_t

	ret := C.camera_grab_rgb(sc.ctx, &data, &width, &height, &size)
	if ret != 0 {
		return nil, 0, 0, fmt.Errorf("camera_grab_rgb failed (code %d)", int(ret))
	}

	// Copy from C memory into Go-managed slice.
	goData := C.GoBytes(unsafe.Pointer(data), C.int(size))
	return goData, int(width), int(height), nil
}

// stop ends acquisition.
func (sc *spinnakerCamera) stop() {
	C.camera_stop(sc.ctx)
}

// destroy releases all resources.
func (sc *spinnakerCamera) destroy() {
	if sc.ctx != nil {
		C.camera_destroy(sc.ctx)
		sc.ctx = nil
	}
}
