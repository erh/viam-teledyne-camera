package flircamera

import "fmt"

// Config holds the configuration for the FLIR Blackfly camera.
type Config struct {
	SerialNumber string  `json:"serial_number"` // optional; auto-detect if empty
	ExposureUs   float64 `json:"exposure_us"`   // 0 = auto
	GainDb       float64 `json:"gain_db"`       // 0 = auto
}

// Validate checks that the config is valid.
func (c *Config) Validate(path string) ([]string, []string, error) {
	if c.ExposureUs < 0 {
		return nil, nil, fmt.Errorf("exposure_us must be non-negative")
	}
	if c.GainDb < 0 {
		return nil, nil, fmt.Errorf("gain_db must be non-negative")
	}
	return nil, nil, nil
}
