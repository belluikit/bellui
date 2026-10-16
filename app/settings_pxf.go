package app

import (
	"github.com/belluikit/bellui/internal/settings/settingspb"
	"github.com/trendvidia/protowire-go/encoding/pxf"
)

// MarshalPXF encodes the global settings to PXF bytes — the textual format of the
// settings.pxf store that replaced the legacy settings.json (pre-rebrand tracker #872, mirroring the
// preferences.pxf migration in pre-rebrand tracker #824). It maps SettingsSchema field-for-field via
// the settingspb.Settings proto. Exported because the writer lives in a separate
// package (cmd/bellui_settings) from the reader (app).
func (sc *SettingsSchema) MarshalPXF() ([]byte, error) {
	return pxf.Marshal(&settingspb.Settings{
		Theme:             sc.ThemeName,
		Scale:             sc.Scale,
		PrimaryColor:      sc.PrimaryColor,
		CloudName:         sc.CloudName,
		CloudConfig:       sc.CloudConfig,
		DisableAnimations: sc.DisableAnimations,
	})
}

// UnmarshalPXF decodes PXF settings bytes into the schema. An error means the bytes
// are not valid PXF; the caller treats it like any other load failure and keeps the
// zero-value defaults.
func (sc *SettingsSchema) UnmarshalPXF(data []byte) error {
	var msg settingspb.Settings
	if err := pxf.Unmarshal(data, &msg); err != nil {
		return err
	}
	sc.ThemeName = msg.GetTheme()
	sc.Scale = msg.GetScale()
	sc.PrimaryColor = msg.GetPrimaryColor()
	sc.CloudName = msg.GetCloudName()
	sc.CloudConfig = msg.GetCloudConfig()
	sc.DisableAnimations = msg.GetDisableAnimations()
	return nil
}
