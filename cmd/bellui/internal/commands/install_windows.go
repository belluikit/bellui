package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-ole/go-ole"
	"github.com/go-ole/go-ole/oleutil"
	"golang.org/x/sys/windows/registry"
)

func linkToStartMenu(path, name string) error {
	ole.CoInitializeEx(0, ole.COINIT_APARTMENTTHREADED|ole.COINIT_SPEED_OVER_MEMORY)
	oleShellObject, err := oleutil.CreateObject("WScript.Shell")
	if err != nil {
		return err
	}
	defer oleShellObject.Release()
	wshell, err := oleShellObject.QueryInterface(ole.IID_IDispatch)
	if err != nil {
		return err
	}
	defer wshell.Release()

	home, _ := os.UserHomeDir()
	dst := filepath.Join(home, "AppData", "Roaming", "Microsoft", "Windows", "Start Menu", "Programs", name+".lnk")
	cs, err := oleutil.CallMethod(wshell, "CreateShortcut", dst)
	if err != nil {
		return err
	}
	dispatch := cs.ToIDispatch()

	oleutil.PutProperty(dispatch, "TargetPath", path)
	_, err = oleutil.CallMethod(dispatch, "Save")
	return err
}

func postInstall(i *Installer) error {
	p := i.Packager
	appName := p.Name
	appExe := p.Name
	if filepath.Ext(p.Name) == ".exe" {
		appName = p.Name[:len(p.Name)-4]
	} else {
		appExe = appExe + ".exe"
	}
	exePath := filepath.Join(i.installDir, appExe)
	if err := registerFileHandlers(p, appName, exePath); err != nil {
		return err
	}
	return linkToStartMenu(exePath, appName)
}

// registerFileHandlers writes the per-user registry keys that ask Windows to
// route file associations and custom URL schemes to the installed executable.
// It writes under HKCU\Software\Classes so no elevation is required.
func registerFileHandlers(p *Packager, appName, exePath string) error {
	command := fmt.Sprintf("\"%s\" \"%%1\"", exePath)

	progIDBase := p.AppID
	if progIDBase == "" {
		progIDBase = appName
	}
	progIDBase = sanitizeProgID(progIDBase)

	for _, assoc := range p.FileAssociations {
		description := assoc.Description
		if description == "" {
			description = appName + " document"
		}
		for _, ext := range assoc.Extensions {
			ext = strings.TrimPrefix(ext, ".")
			if ext == "" {
				continue
			}
			progID := progIDBase + "." + ext
			if err := setClassKey(`.`+ext, progID); err != nil {
				return err
			}
			if err := setClassKey(progID, description); err != nil {
				return err
			}
			if assoc.MimeType != "" {
				if err := setClassValue(`.`+ext, "Content Type", assoc.MimeType); err != nil {
					return err
				}
			}
			if err := setClassKey(progID+`\shell\open\command`, command); err != nil {
				return err
			}
		}
	}

	for _, scheme := range p.URLSchemes {
		if scheme == "" {
			continue
		}
		if err := setClassKey(scheme, "URL:"+scheme+" Protocol"); err != nil {
			return err
		}
		if err := setClassValue(scheme, "URL Protocol", ""); err != nil {
			return err
		}
		if err := setClassKey(scheme+`\shell\open\command`, command); err != nil {
			return err
		}
	}

	return nil
}

// setClassKey creates HKCU\Software\Classes\<subKey> and sets its default value.
func setClassKey(subKey, defaultValue string) error {
	return setClassValue(subKey, "", defaultValue)
}

// setClassValue creates HKCU\Software\Classes\<subKey> and sets the named value
// (use "" for the key's default value).
func setClassValue(subKey, name, value string) error {
	k, _, err := registry.CreateKey(registry.CURRENT_USER, `Software\Classes\`+subKey, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("failed to create registry key %q: %w", subKey, err)
	}
	defer k.Close()
	if err := k.SetStringValue(name, value); err != nil {
		return fmt.Errorf("failed to set registry value %q on %q: %w", name, subKey, err)
	}
	return nil
}

// sanitizeProgID strips characters that are not valid in a registry ProgID.
func sanitizeProgID(id string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '.':
			return r
		default:
			return '_'
		}
	}, id)
}
