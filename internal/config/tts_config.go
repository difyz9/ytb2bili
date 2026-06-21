package config

import "time"



type TTSConfig struct {
	Provider       string  `toml:"provider" json:"provider,omitempty"`
	Search         string  `toml:"search" json:"search,omitempty"`
	Voice          string  `toml:"voice" json:"voice,omitempty"`
	Locale         string  `toml:"locale" json:"locale,omitempty"`
	Format         string  `toml:"format" json:"format,omitempty"`
	TimeoutSeconds int     `toml:"timeout_seconds" json:"timeout_seconds,omitempty"`
	Rate           float64 `toml:"rate" json:"rate,omitempty"`
	Volume         float64 `toml:"volume" json:"volume,omitempty"`
	Pitch          float64 `toml:"pitch" json:"pitch,omitempty"`

}


func (c TTSConfig) TimeoutDuration() time.Duration {
	if c.TimeoutSeconds <= 0 {
		return 0
	}
	return time.Duration(c.TimeoutSeconds) * time.Second
}