package service

import (
	"fmt"

	"github.com/moduforge/backend/internal/domain"
)

func (s *SQLiteMarketService) GetModuleDemo(slug string) (*domain.ModuleDemo, error) {
	mod, err := s.GetModule(slug)
	if err != nil {
		return nil, fmt.Errorf("module not found: %w", err)
	}

	demo := &domain.ModuleDemo{
		Slug:  mod.Slug,
		Title: mod.Title,
	}

	switch mod.Category {
	case "system":
		demo.Props = []domain.DemoProp{
			{Path: "system/build.prop", Prop: "ro.build.version.sdk", Before: "34", After: "35"},
			{Path: "system/build.prop", Prop: "ro.build.version.release", Before: "14", After: "15"},
			{Path: "system/build.prop", Prop: "ro.product.model", Before: "Pixel 8", After: "Pixel 8 Pro"},
		}
		demo.Files = []string{"/data/adb/modules/" + mod.Slug + "/system/build.prop"}
		demo.SimulatedOutput = "  MKSTOCK 1.0.0 (2024-01-15)\n" +
			"  - Installing Magisk Module: " + mod.Title + "\n" +
			"  - Extracting module files...\n" +
			"  - Mounting system overlay...\n" +
			"  - Patching system/build.prop (3 props)\n" +
			"    · ro.build.version.sdk: 34 → 35\n" +
			"    · ro.build.version.release: 14 → 15\n" +
			"    · ro.product.model: Pixel 8 → Pixel 8 Pro\n" +
			"  - Setting permissions...\n" +
			"  - Done! Reboot recommended."
	case "ui":
		demo.Props = []domain.DemoProp{
			{Path: "system/build.prop", Prop: "ro.config.miui_round_icon", Before: "0", After: "1"},
			{Path: "system/build.prop", Prop: "persist.sys.ui.hw", Before: "0", After: "1"},
		}
		demo.Files = []string{"/data/adb/modules/" + mod.Slug + "/system/media/theme/default"}
		demo.SimulatedOutput = "  MKSTOCK 1.0.0 (2024-01-15)\n" +
			"  - Installing Magisk Module: " + mod.Title + "\n" +
			"  - Extracting module files...\n" +
			"  - Deploying UI theme overlay...\n" +
			"  - Patching system/build.prop (2 props)\n" +
			"    · ro.config.miui_round_icon: 0 → 1\n" +
			"    · persist.sys.ui.hw: 0 → 1\n" +
			"  - Applying icon pack...\n" +
			"  - Done! Reboot recommended."
	case "audio":
		demo.Props = []domain.DemoProp{
			{Path: "system/build.prop", Prop: "ro.config.media_vol_steps", Before: "15", After: "30"},
			{Path: "system/build.prop", Prop: "ro.config.vc_call_vol_steps", Before: "7", After: "15"},
			{Path: "system/etc/audio_policy.conf", Prop: "sampling_rates", Before: "48000", After: "96000"},
		}
		demo.Files = []string{"/data/adb/modules/" + mod.Slug + "/system/etc/audio_policy.conf"}
		demo.SimulatedOutput = "  MKSTOCK 1.0.0 (2024-01-15)\n" +
			"  - Installing Magisk Module: " + mod.Title + "\n" +
			"  - Extracting module files...\n" +
			"  - Patching audio policy...\n" +
			"  - Modifying system/build.prop (2 props)\n" +
			"    · ro.config.media_vol_steps: 15 → 30\n" +
			"    · ro.config.vc_call_vol_steps: 7 → 15\n" +
			"  - Patching audio_policy.conf\n" +
			"    · sampling_rates: 48000 → 96000\n" +
			"  - Done! Reboot recommended."
	case "display":
		demo.Props = []domain.DemoProp{
			{Path: "system/build.prop", Prop: "ro.sf.lcd_density", Before: "440", After: "380"},
			{Path: "system/build.prop", Prop: "persist.sys.powersaving", Before: "1", After: "0"},
		}
		demo.Files = []string{"/data/adb/modules/" + mod.Slug + "/system/etc/display_hal.conf"}
		demo.SimulatedOutput = "  MKSTOCK 1.0.0 (2024-01-15)\n" +
			"  - Installing Magisk Module: " + mod.Title + "\n" +
			"  - Extracting module files...\n" +
			"  - Configuring display settings...\n" +
			"  - Patching system/build.prop (2 props)\n" +
			"    · ro.sf.lcd_density: 440 → 380\n" +
			"    · persist.sys.powersaving: 1 → 0\n" +
			"  - Deploying display HAL config...\n" +
			"  - Done! Reboot recommended."
	default:
		demo.Props = []domain.DemoProp{
			{Path: "system/build.prop", Prop: "ro.config.custom_prop", Before: "(unset)", After: mod.Title},
		}
		demo.Files = []string{"/data/adb/modules/" + mod.Slug + "/system/etc/" + mod.Slug + ".conf"}
		demo.SimulatedOutput = "  MKSTOCK 1.0.0 (2024-01-15)\n" +
			"  - Installing Magisk Module: " + mod.Title + "\n" +
			"  - Extracting module files...\n" +
			"  - Applying configuration...\n" +
			"  - Patching system/build.prop\n" +
			"    · ro.config.custom_prop: (unset) → " + mod.Title + "\n" +
			"  - Deploying config files...\n" +
			"  - Done! Reboot recommended."
	}

	return demo, nil
}
