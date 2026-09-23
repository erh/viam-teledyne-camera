package flircamera

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"sync"

	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/data"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils"
)

// Model is the full model triplet for this camera: erh:flir-spinnaker:blackfly.
var Model = resource.NewModel("erh", "flir-spinnaker", "blackfly")

func init() {
	resource.RegisterComponent(
		camera.API,
		Model,
		resource.Registration[camera.Camera, *Config]{Constructor: newCamera},
	)
}

type flirCamera struct {
	resource.Named
	resource.AlwaysRebuild

	logger logging.Logger
	mu     sync.Mutex
	cam    *spinnakerCamera
}

func newCamera(
	ctx context.Context,
	_ resource.Dependencies,
	conf resource.Config,
	logger logging.Logger,
) (camera.Camera, error) {
	cfg, err := resource.NativeConfig[*Config](conf)
	if err != nil {
		return nil, err
	}

	cam, err := newSpinnakerCamera(cfg.SerialNumber)
	if err != nil {
		return nil, fmt.Errorf("open camera: %w", err)
	}

	autoExposure := cfg.ExposureUs == 0
	autoGain := cfg.GainDb == 0

	if err := cam.configure(cfg.ExposureUs, cfg.GainDb, autoExposure, autoGain); err != nil {
		cam.destroy()
		return nil, fmt.Errorf("configure camera: %w", err)
	}

	if err := cam.start(); err != nil {
		cam.destroy()
		return nil, fmt.Errorf("start camera: %w", err)
	}

	logger.Infof("FLIR Spinnaker camera opened (serial=%q)", cfg.SerialNumber)

	return &flirCamera{
		Named:  conf.ResourceName().AsNamed(),
		logger: logger,
		cam:    cam,
	}, nil
}

// rgbImage is a minimal image.Image backed by raw RGB bytes.
type rgbImage struct {
	data   []byte
	width  int
	height int
}

func (r *rgbImage) ColorModel() color.Model { return color.NRGBAModel }
func (r *rgbImage) Bounds() image.Rectangle { return image.Rect(0, 0, r.width, r.height) }
func (r *rgbImage) At(x, y int) color.Color {
	i := (y*r.width + x) * 3
	return color.NRGBA{R: r.data[i], G: r.data[i+1], B: r.data[i+2], A: 0xFF}
}

func (fc *flirCamera) Images(ctx context.Context, filterSourceNames []string, extra map[string]interface{},
) ([]camera.NamedImage, resource.ResponseMetadata, error) {
	fc.mu.Lock()
	defer fc.mu.Unlock()

	if fc.cam == nil {
		return nil, resource.ResponseMetadata{}, fmt.Errorf("camera not initialized")
	}

	rgbData, width, height, err := fc.cam.grabRGB()
	if err != nil {
		return nil, resource.ResponseMetadata{}, fmt.Errorf("grab frame: %w", err)
	}

	img := &rgbImage{data: rgbData, width: width, height: height}

	namedImg, err := camera.NamedImageFromImage(img, "color", utils.MimeTypeJPEG, data.Annotations{})
	if err != nil {
		return nil, resource.ResponseMetadata{}, err
	}

	return []camera.NamedImage{namedImg}, resource.ResponseMetadata{}, nil
}

func (fc *flirCamera) Image(ctx context.Context, mimeType string, extra map[string]interface{},
) ([]byte, camera.ImageMetadata, error) {
	return camera.GetImageFromGetImages(ctx, nil, fc, extra, nil)
}

func (fc *flirCamera) NextPointCloud(ctx context.Context, extra map[string]interface{}) (pointcloud.PointCloud, error) {
	return nil, fmt.Errorf("point clouds not supported")
}

func (fc *flirCamera) Properties(ctx context.Context) (camera.Properties, error) {
	return camera.Properties{
		SupportsPCD: false,
		MimeTypes:   []string{utils.MimeTypeJPEG},
	}, nil
}

func (fc *flirCamera) Geometries(ctx context.Context, extra map[string]interface{}) ([]spatialmath.Geometry, error) {
	return nil, nil
}

func (fc *flirCamera) Close(ctx context.Context) error {
	fc.mu.Lock()
	defer fc.mu.Unlock()

	if fc.cam != nil {
		fc.cam.stop()
		fc.cam.destroy()
		fc.cam = nil
		fc.logger.Info("FLIR Spinnaker camera closed")
	}
	return nil
}
