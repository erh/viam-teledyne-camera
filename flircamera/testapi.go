package flircamera

// TestCamera exposes the spinnakerCamera for standalone testing.
type TestCamera struct {
	cam *spinnakerCamera
}

func TestNewCamera(serial string) (*TestCamera, error) {
	cam, err := newSpinnakerCamera(serial)
	if err != nil {
		return nil, err
	}
	return &TestCamera{cam: cam}, nil
}

func (tc *TestCamera) Configure(exposureUs, gainDb float64, autoExposure, autoGain bool) error {
	return tc.cam.configure(exposureUs, gainDb, autoExposure, autoGain)
}

func (tc *TestCamera) Start() error {
	return tc.cam.start()
}

func (tc *TestCamera) GrabRGB() ([]byte, int, int, error) {
	return tc.cam.grabRGB()
}

func (tc *TestCamera) Stop() {
	tc.cam.stop()
}

func (tc *TestCamera) Close() {
	tc.cam.destroy()
}
