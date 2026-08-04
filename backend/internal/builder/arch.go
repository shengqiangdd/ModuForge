package builder

import "fmt"

// Arch represents a target architecture for cross-compilation.
type Arch string

const (
	ArchARM64  Arch = "arm64"
	ArchARM    Arch = "arm"
	ArchX86_64 Arch = "x86_64"
)

// SupportedArchitectures defines all architectures we can build for.
var SupportedArchitectures = []ArchInfo{
	{
		ID:         "arm64",
		Name:       "ARM64 (aarch64)",
		Goarch:     "arm64",
		RustTarget: "aarch64-linux-android",
		GOARM:      "",
		Icon:       "smartphone",
		MinAPI:     21,
		Default:    true,
	},
	{
		ID:         "arm",
		Name:       "ARM (armv7)",
		Goarch:     "arm",
		RustTarget: "armv7-linux-androideabi",
		GOARM:      "7",
		Icon:       "phone_android",
		MinAPI:     16,
		Default:    false,
	},
	{
		ID:         "x86_64",
		Name:       "x86_64 (x86)",
		Goarch:     "amd64",
		RustTarget: "x86_64-linux-android",
		GOARM:      "",
		Icon:       "computer",
		MinAPI:     21,
		Default:    false,
	},
}

// ArchInfo holds metadata for a supported architecture.
type ArchInfo struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Goarch     string `json:"goarch"`
	RustTarget string `json:"rust_target"`
	GOARM      string `json:"goarm,omitempty"`
	Icon       string `json:"icon"`
	MinAPI     int    `json:"min_api"`
	Default    bool   `json:"default"`
}

// GetArchInfo returns the ArchInfo for a given architecture ID.
func GetArchInfo(archID string) (*ArchInfo, error) {
	for _, a := range SupportedArchitectures {
		if a.ID == archID {
			return &a, nil
		}
	}
	return nil, fmt.Errorf("unsupported architecture: %s", archID)
}

// DefaultArch returns the default architecture.
func DefaultArch() ArchInfo {
	return SupportedArchitectures[0] // arm64
}

// ValidateArch checks if an architecture string is valid.
func ValidateArch(arch string) bool {
	for _, a := range SupportedArchitectures {
		if a.ID == arch {
			return true
		}
	}
	return false
}

// NormalizeArch normalizes an architecture string to a valid ID,
// returning the default if invalid.
func NormalizeArch(arch string) string {
	if ValidateArch(arch) {
		return arch
	}
	return "arm64"
}
