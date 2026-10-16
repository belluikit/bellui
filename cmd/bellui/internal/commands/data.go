package commands

import "github.com/belluikit/bellui/cmd/bellui/internal/metadata"

type appData struct {
	icon, Name        string
	AppID, AppVersion string
	AppBuild          int
	ResGoString       string
	Release, rawIcon  bool
	AdaptiveIcon      *metadata.AdaptiveIcon
	CustomMetadata    map[string]string
	Migrations        map[string]bool
	CanOpen           *metadata.CanOpen
	FileAssociations  []metadata.FileAssociation
	URLSchemes        []string
	// IOSUsageDescriptions holds the iOS Info.plist usage strings from
	// manifest.pxf (details.ios_usage_descriptions), injected into the generated
	// Info.plist by packageIOS. Empty on non-iOS packaging.
	IOSUsageDescriptions map[string]string
	// AssociatedDomains holds the Universal-Link domains from manifest.pxf
	// (details.associated_domains); packageIOS emits them as an Associated
	// Domains entitlement. Empty on non-iOS packaging.
	AssociatedDomains []string
}

// fileMimeTypes returns the distinct, non-empty MIME types declared across all
// file associations, preserving declaration order.
func (a *appData) fileMimeTypes() []string {
	var mimes []string
	seen := map[string]bool{}
	for _, assoc := range a.FileAssociations {
		if assoc.MimeType == "" || seen[assoc.MimeType] {
			continue
		}
		seen[assoc.MimeType] = true
		mimes = append(mimes, assoc.MimeType)
	}
	return mimes
}

func (a *appData) appendCustomMetadata(fromFile map[string]string) {
	if a.CustomMetadata == nil {
		a.CustomMetadata = map[string]string{}
	}

	for key, value := range fromFile {
		_, ok := a.CustomMetadata[key]
		if ok {
			continue
		}
		a.CustomMetadata[key] = value
	}
}

func (a *appData) mergeMetadata(data *metadata.BelluiApp) {
	if a.icon == "" {
		a.icon = data.Details.Icon
	}
	if a.Name == "" {
		a.Name = data.Details.Name
	}
	if a.AppID == "" {
		a.AppID = data.Details.ID
	}
	if a.AppVersion == "" {
		a.AppVersion = data.Details.Version
	}
	if a.AppBuild == 0 {
		a.AppBuild = data.Details.Build
	}
	a.AdaptiveIcon = data.AdaptiveIcon

	if a.Release {
		a.appendCustomMetadata(data.Release)
	} else {
		a.appendCustomMetadata(data.Development)
	}
	a.Migrations = data.Migrations
	a.CanOpen = data.CanOpen
	a.FileAssociations = data.Details.FileAssociations
	a.URLSchemes = data.Details.URLSchemes
	a.IOSUsageDescriptions = data.Details.IOSUsageDescriptions
	a.AssociatedDomains = data.Details.AssociatedDomains
}
