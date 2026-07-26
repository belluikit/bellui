package metadata

import "github.com/belluikit/bellui/cmd/bellui/internal/metadata/belluiapppb"

// toProto converts a BelluiApp into its generated protobuf message so it can be
// marshaled to the manifest.pxf textual PXF format.
func (f *BelluiApp) toProto() *belluiapppb.BelluiApp {
	if f == nil {
		return nil
	}

	msg := &belluiapppb.BelluiApp{
		Website:     f.Website,
		Description: f.Description,
		Details: &belluiapppb.AppDetails{
			Icon:                 f.Details.Icon,
			Name:                 f.Details.Name,
			Id:                   f.Details.ID,
			Version:              f.Details.Version,
			Build:                int32(f.Details.Build),
			FileAssociations:     fileAssociationsToProto(f.Details.FileAssociations),
			UrlSchemes:           f.Details.URLSchemes,
			IosUsageDescriptions: f.Details.IOSUsageDescriptions,
			AssociatedDomains:    f.Details.AssociatedDomains,
		},
		Development: f.Development,
		Release:     f.Release,
		Languages:   f.Languages,
		Migrations:  f.Migrations,
	}

	if f.AdaptiveIcon != nil {
		msg.AdaptiveIcon = &belluiapppb.AdaptiveIcon{
			Foreground: f.AdaptiveIcon.Foreground,
			Background: f.AdaptiveIcon.Background,
			Monochrome: f.AdaptiveIcon.Monochrome,
		}
	}
	if f.Source != nil {
		msg.Source = &belluiapppb.AppSource{Repo: f.Source.Repo, Dir: f.Source.Dir}
	}
	if f.CanOpen != nil {
		msg.CanOpen = &belluiapppb.CanOpen{MimeTypes: f.CanOpen.MimeTypes}
	}
	if f.LinuxAndBSD != nil {
		msg.LinuxAndBsd = &belluiapppb.LinuxAndBSD{
			GenericName: f.LinuxAndBSD.GenericName,
			Categories:  f.LinuxAndBSD.Categories,
			Comment:     f.LinuxAndBSD.Comment,
			Keywords:    f.LinuxAndBSD.Keywords,
			ExecParams:  f.LinuxAndBSD.ExecParams,
		}
	}

	return msg
}

// fromProto converts a generated protobuf message decoded from manifest.pxf back
// into the BelluiApp struct used throughout the CLI.
func fromProto(msg *belluiapppb.BelluiApp) *BelluiApp {
	f := &BelluiApp{
		Website:     msg.GetWebsite(),
		Description: msg.GetDescription(),
		Development: msg.GetDevelopment(),
		Release:     msg.GetRelease(),
		Languages:   msg.GetLanguages(),
		Migrations:  msg.GetMigrations(),
	}

	if d := msg.GetDetails(); d != nil {
		f.Details = AppDetails{
			Icon:                 d.GetIcon(),
			Name:                 d.GetName(),
			ID:                   d.GetId(),
			Version:              d.GetVersion(),
			Build:                int(d.GetBuild()),
			FileAssociations:     fileAssociationsFromProto(d.GetFileAssociations()),
			URLSchemes:           d.GetUrlSchemes(),
			IOSUsageDescriptions: d.GetIosUsageDescriptions(),
			AssociatedDomains:    d.GetAssociatedDomains(),
		}
	}
	if ai := msg.GetAdaptiveIcon(); ai != nil {
		f.AdaptiveIcon = &AdaptiveIcon{
			Foreground: ai.GetForeground(),
			Background: ai.GetBackground(),
			Monochrome: ai.GetMonochrome(),
		}
	}
	if s := msg.GetSource(); s != nil {
		f.Source = &AppSource{Repo: s.GetRepo(), Dir: s.GetDir()}
	}
	if c := msg.GetCanOpen(); c != nil {
		f.CanOpen = &CanOpen{MimeTypes: c.GetMimeTypes()}
	}
	if l := msg.GetLinuxAndBsd(); l != nil {
		f.LinuxAndBSD = &LinuxAndBSD{
			GenericName: l.GetGenericName(),
			Categories:  l.GetCategories(),
			Comment:     l.GetComment(),
			Keywords:    l.GetKeywords(),
			ExecParams:  l.GetExecParams(),
		}
	}

	return f
}

func fileAssociationsToProto(assocs []FileAssociation) []*belluiapppb.FileAssociation {
	if len(assocs) == 0 {
		return nil
	}

	r := make([]*belluiapppb.FileAssociation, len(assocs))
	for i, a := range assocs {
		r[i] = &belluiapppb.FileAssociation{
			Extensions:  a.Extensions,
			MimeType:    a.MimeType,
			Description: a.Description,
			Role:        a.Role,
			Icon:        a.Icon,
		}
	}
	return r
}

func fileAssociationsFromProto(assocs []*belluiapppb.FileAssociation) []FileAssociation {
	if len(assocs) == 0 {
		return nil
	}

	r := make([]FileAssociation, len(assocs))
	for i, a := range assocs {
		r[i] = FileAssociation{
			Extensions:  a.GetExtensions(),
			MimeType:    a.GetMimeType(),
			Description: a.GetDescription(),
			Role:        a.GetRole(),
			Icon:        a.GetIcon(),
		}
	}
	return r
}
