package bilibili

import (
	"context"

	sdkbili "github.com/difyz9/bilibili-go-sdk/bilibili"
)

// VideoZone re-exports the SDK's static Bilibili video-zone type.
type VideoZone = sdkbili.VideoZone

// ListVideoZones returns the static video-zone tree provided by bilibili-go-sdk.
// The signature stays compatible with existing callers, but no user state is required.
func (s *Service) ListVideoZones(_ context.Context, _ string) ([]VideoZone, error) {
	return sdkbili.GetVideoZones(), nil
}

// ListMainVideoZones returns only the first-level video zones from the SDK.
func (s *Service) ListMainVideoZones() []VideoZone {
	return sdkbili.GetMainVideoZones()
}

// ListAllVideoZones returns the flattened video-zone list from the SDK.
func (s *Service) ListAllVideoZones() []VideoZone {
	return sdkbili.GetAllVideoZones()
}
