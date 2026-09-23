// Package main is the entry point for the FLIR Spinnaker camera module.
package main

import (
	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/module"
	"go.viam.com/rdk/resource"

	"viam-flir-camera/flircamera"
)

func main() {
	module.ModularMain(
		resource.APIModel{API: camera.API, Model: flircamera.Model},
	)
}
