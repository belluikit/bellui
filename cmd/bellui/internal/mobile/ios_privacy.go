package mobile

import "fmt"

// App Store Connect refuses an iOS upload that calls a "required reason" API
// without a privacy manifest declaring why (ITMS-91053, #932). The packager
// writes one into every app, declaring what the toolkit and the Go runtime
// call; the app's ios/PrivacyInfo.xcprivacy adds what its own code calls.
const (
	iosPrivacyManifest = "PrivacyInfo.xcprivacy"
	privacyAPITypesKey = "NSPrivacyAccessedAPITypes"
	privacyAPITypeKey  = "NSPrivacyAccessedAPIType"
	privacyReasonsKey  = "NSPrivacyAccessedAPITypeReasons"
)

// iosRuntimeAPIReasons are the required-reason API categories every bellui
// app calls, whatever its own code does, with the reason each is called for.
// Measured with nm -u on a one-label app built for ios/arm64 (#932): the Go
// runtime and package os import stat, fstat, lstat and fstatat, on files in
// the app's own container (C617.1), and the monotonic clock imports
// mach_absolute_time, to measure elapsed time (35F9.1). A plain Go program
// imports the same set; TestIOSRuntimeAPIReasonsCoverGo builds one with the
// installed Go and fails when it imports a category this list lacks.
var iosRuntimeAPIReasons = []struct{ Category, Reason string }{
	{"NSPrivacyAccessedAPICategoryFileTimestamp", "C617.1"},
	{"NSPrivacyAccessedAPICategorySystemBootTime", "35F9.1"},
}

// iosPrivacyInfo builds the app's PrivacyInfo.xcprivacy: the toolkit's
// required-reason declarations, merged with the app's ios/PrivacyInfo.xcprivacy.
func iosPrivacyInfo(appDir string) ([]byte, error) {
	fragment, err := readIOSFragment(appDir, iosPrivacyManifest)
	if err != nil {
		return nil, err
	}
	m, err := mergePrivacyInfo(fragment)
	if err != nil {
		return nil, err
	}
	return encodePlist(m), nil
}

// mergePrivacyInfo merges the app's privacy manifest fragment into the
// toolkit's. NSPrivacyAccessedAPITypes is merged per category, the reasons of
// both sides kept once each; every other key (NSPrivacyTracking, the tracking
// domains, the collected data types) only the app can know, so it is taken
// from the fragment as written and the toolkit never sets it.
func mergePrivacyInfo(fragment *plistValue) (*plistValue, error) {
	types := &plistValue{Kind: "array"}
	reasons := map[string]*plistValue{}
	add := func(category, reason string) {
		r := reasons[category]
		if r == nil {
			r = &plistValue{Kind: "array"}
			reasons[category] = r
			entry := &plistValue{Kind: "dict"}
			entry.set(privacyAPITypeKey, plistString(category))
			entry.set(privacyReasonsKey, r)
			types.Items = append(types.Items, entry)
		}
		for _, it := range r.Items {
			if it.Text == reason {
				return
			}
		}
		r.Items = append(r.Items, plistString(reason))
	}
	for _, r := range iosRuntimeAPIReasons {
		add(r.Category, r.Reason)
	}

	out := &plistValue{Kind: "dict"}
	out.set(privacyAPITypesKey, types)
	if fragment == nil {
		return out, nil
	}
	where := iosExtrasDir + "/" + iosPrivacyManifest
	for i, k := range fragment.Keys {
		v := fragment.Vals[i]
		if k != privacyAPITypesKey {
			out.set(k, v)
			continue
		}
		if v.Kind != "array" {
			return nil, fmt.Errorf("%s: %s must be an <array>", where, privacyAPITypesKey)
		}
		for j, entry := range v.Items {
			if entry.Kind != "dict" {
				return nil, fmt.Errorf("%s: %s item %d must be a <dict>", where, privacyAPITypesKey, j)
			}
			category := entry.get(privacyAPITypeKey)
			if category == nil || category.Kind != "string" || category.Text == "" {
				return nil, fmt.Errorf("%s: %s item %d needs a %s string", where, privacyAPITypesKey, j, privacyAPITypeKey)
			}
			rs := entry.get(privacyReasonsKey)
			if rs == nil || rs.Kind != "array" || len(rs.Items) == 0 {
				return nil, fmt.Errorf("%s: %s %s needs a non-empty %s array", where, privacyAPITypeKey, category.Text, privacyReasonsKey)
			}
			for _, r := range rs.Items {
				if r.Kind != "string" || r.Text == "" {
					return nil, fmt.Errorf("%s: %s %s: every reason must be a <string>", where, privacyAPITypeKey, category.Text)
				}
				add(category.Text, r.Text)
			}
		}
	}
	return out, nil
}
