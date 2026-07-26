// Copyright 2015 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package mobile

import (
	"encoding/xml"
	"errors"
)

type manifestXML struct {
	Activities []activityXML `xml:"application>activity"`
}

type activityXML struct {
	Name     string        `xml:"name,attr"`
	MetaData []metaDataXML `xml:"meta-data"`
}

type metaDataXML struct {
	Name  string `xml:"name,attr"`
	Value string `xml:"value,attr"`
}

// manifestLibName parses the AndroidManifest.xml and finds the library
// name of the NativeActivity.
func manifestLibName(data []byte) (string, error) {
	manifest := new(manifestXML)
	if err := xml.Unmarshal(data, manifest); err != nil {
		return "", err
	}
	// The app may declare several activities; find the GoNativeActivity that
	// hosts the gomobile library (others may be app-provided, e.g. permission
	// shims).
	var activity *activityXML
	for i := range manifest.Activities {
		if manifest.Activities[i].Name == "org.golang.app.GoNativeActivity" {
			activity = &manifest.Activities[i]
			break
		}
	}
	if activity == nil {
		return "", errors.New("can only build an .apk for org.golang.app.GoNativeActivity; none declared in AndroidManifest.xml")
	}
	libName := ""
	for _, md := range activity.MetaData {
		if md.Name == "android.app.lib_name" {
			libName = md.Value
			break
		}
	}
	if libName == "" {
		return "", errors.New("AndroidManifest.xml missing meta-data android.app.lib_name")
	}
	return libName, nil
}
