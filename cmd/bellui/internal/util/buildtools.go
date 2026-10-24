package util

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// windowsBuildToolExt is the file extension each build-tools program bellui
// runs carries on Windows. The native tools are .exe; d8 and apksigner are
// Java programs, shipped as a shell script elsewhere and as a .bat wrapper on
// Windows. Joining the bare name onto the build-tools directory finds nothing
// there (#674).
var windowsBuildToolExt = map[string]string{
	"aapt":      ".exe",
	"aapt2":     ".exe",
	"dexdump":   ".exe",
	"zipalign":  ".exe",
	"d8":        ".bat",
	"apksigner": ".bat",
}

// BuildToolFile returns the file name of the Android build-tools program name
// on the host OS: "d8.bat" on Windows, "d8" elsewhere. It panics on a program
// it does not know, so a new tool gets its Windows extension recorded above.
func BuildToolFile(name string) string {
	return buildToolFile(name, runtime.GOOS)
}

func buildToolFile(name, goos string) string {
	ext, ok := windowsBuildToolExt[name]
	if !ok {
		panic("util: unknown Android build-tools program " + name)
	}
	if goos == "windows" {
		return name + ext
	}
	return name
}

// AndroidBuildTool returns the path of the Android build-tools program name
// (d8, aapt2, apksigner, zipalign, dexdump, aapt) in AndroidBuildToolsPath,
// with the host OS's file extension, or an error when it is not installed
// there. Run the result with Command, which handles the .bat wrappers.
func AndroidBuildTool(name string) (string, error) {
	return androidBuildTool(AndroidBuildToolsPath(), name, runtime.GOOS)
}

func androidBuildTool(dir, name, goos string) (string, error) {
	p := filepath.Join(dir, buildToolFile(name, goos))
	if !Exists(p) {
		return "", fmt.Errorf("%s not found in Android SDK build-tools (%s)", name, dir)
	}
	return p, nil
}

// Command returns the exec.Cmd that runs the program at path with args. It is
// exec.Command, except for a Windows batch file (.bat, .cmd — d8 and apksigner
// in the build-tools): Windows runs one through cmd.exe, which does not split
// a command line the way exec.Command quotes it, and expands %VAR% inside it.
// Command then spells the cmd.exe line itself (see batchCmdLine) and refuses
// arguments that cmd.exe could not pass through unchanged.
func Command(path string, args ...string) (*exec.Cmd, error) {
	if !isBatchFile(path) {
		return exec.Command(path, args...), nil
	}
	return batchCommand(path, args)
}

func isBatchFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".bat" || ext == ".cmd"
}

// batchUnsafe are the characters cmd.exe does not leave alone inside a quoted
// argument: % and ! expand variables, " ends the quoting, ^ is doubled by the
// CALL the build-tools wrappers use to start java, and a line break ends the
// command.
const batchUnsafe = "%!^\"\r\n\x00"

// batchCmdLine returns the command line, after the program name, that runs
// the batch file bat with args through `cmd.exe /s /c`. /s strips exactly the
// outer pair of quotes from what follows /c, so the line is
// ""bat" "arg1" "arg2"" and every argument stays quoted: spaces, & | < > ( )
// and = ; , are literal inside quotes. The wrapper hands the quoted arguments
// to java.exe, which reads \" as an escaped quote, so trailing backslashes are
// doubled. An argument holding a character from batchUnsafe is refused rather
// than escaped: no escape survives both cmd.exe and the wrapper's CALL.
func batchCmdLine(bat string, args []string) (string, error) {
	var b strings.Builder
	b.WriteString(`/d /v:off /s /c "`)
	for i, a := range append([]string{bat}, args...) {
		if strings.ContainsAny(a, batchUnsafe) {
			return "", fmt.Errorf("cannot pass %q to %s: cmd.exe would change an argument containing any of %%, !, ^, \" or a line break", a, filepath.Base(bat))
		}
		if i > 0 {
			b.WriteByte(' ')
		}
		b.WriteByte('"')
		b.WriteString(a)
		// Double the trailing backslashes so they do not escape the quote.
		for j := len(a) - 1; j >= 0 && a[j] == '\\'; j-- {
			b.WriteByte('\\')
		}
		b.WriteByte('"')
	}
	b.WriteByte('"')
	return b.String(), nil
}
