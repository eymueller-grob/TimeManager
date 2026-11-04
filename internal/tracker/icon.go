package tracker

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"image"
	"image/png"
	"strings"
)

const (
	projectIconPrefix = "data:image/png;base64,"
	// MaxProjectIconSize is the largest allowed icon width and height in pixels.
	MaxProjectIconSize = 128
	// MaxProjectIconLength bounds the stored data URL so data.json stays small.
	MaxProjectIconLength = 64 * 1024
)

// ValidateProjectIcon accepts an empty icon or a PNG data URL of at most
// MaxProjectIconSize pixels in each dimension.
func ValidateProjectIcon(icon string) error {
	if icon == "" {
		return nil
	}
	_, err := DecodeProjectIcon(icon)
	return err
}

// DecodeProjectIcon returns the image stored in a project icon data URL.
func DecodeProjectIcon(icon string) (image.Image, error) {
	if icon == "" {
		return nil, errors.New("project has no icon")
	}
	if len(icon) > MaxProjectIconLength {
		return nil, fmt.Errorf("project icon is too large; use an image up to %d KB", MaxProjectIconLength/1024)
	}
	encoded, ok := strings.CutPrefix(icon, projectIconPrefix)
	if !ok {
		return nil, errors.New("project icon must be a PNG data URL")
	}
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("project icon is not valid base64: %w", err)
	}
	config, err := png.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("project icon is not a valid PNG image: %w", err)
	}
	if config.Width < 1 || config.Height < 1 || config.Width > MaxProjectIconSize || config.Height > MaxProjectIconSize {
		return nil, fmt.Errorf("project icon must be between 1 and %d pixels wide and high", MaxProjectIconSize)
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("project icon is not a valid PNG image: %w", err)
	}
	return img, nil
}
