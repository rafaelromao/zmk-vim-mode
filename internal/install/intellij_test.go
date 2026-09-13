package install

import (
	"archive/zip"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestParseBuildTxt(t *testing.T) {
	for _, tc := range []struct {
		in         string
		wantCode   string
		wantConfig string
		wantErr    bool
	}{
		{in: "IU-262.10315.125", wantCode: "IU", wantConfig: "IntelliJIdea2026.2"},
		{in: "IU-251.23774.435\n", wantCode: "IU", wantConfig: "IntelliJIdea2025.1"},
		{in: "IC-262.10315.125", wantCode: "IC", wantConfig: "IdeaIC2026.2"},
		{in: "GO-262.10315.125", wantCode: "GO", wantConfig: "GoLand2026.2"},
		// Fleet is not an IdeaVim host, and a bare number is not a build.
		{in: "FL-262.1", wantCode: "FL", wantErr: true},
		{in: "262.10315.125", wantErr: true},
		{in: "IU-26.1", wantCode: "IU", wantErr: true},
	} {
		code, config, err := ParseBuildTxt(tc.in)
		if (err != nil) != tc.wantErr {
			t.Errorf("ParseBuildTxt(%q): err = %v, want error = %v", tc.in, err, tc.wantErr)
			continue
		}
		if code != tc.wantCode {
			t.Errorf("ParseBuildTxt(%q): code = %q, want %q", tc.in, code, tc.wantCode)
		}
		if !tc.wantErr && config != tc.wantConfig {
			t.Errorf("ParseBuildTxt(%q): config = %q, want %q", tc.in, config, tc.wantConfig)
		}
	}
}

func TestSinceBuild(t *testing.T) {
	for in, want := range map[string]string{
		"IU-262.10315.125": "262",
		"IC-251.1":         "251",
		"nonsense":         "",
	} {
		if got := (IntelliJIDE{Build: in}).SinceBuild(); got != want {
			t.Errorf("SinceBuild(%q) = %q, want %q", in, got, want)
		}
	}
}

// The two roots differ on Linux and coincide on macOS, which is exactly the
// distinction a hand-written path gets wrong.
func TestPluginsDirFor(t *testing.T) {
	got := PluginsDirFor("/home/u", "IntelliJIdea2026.2")
	want := "/home/u/.local/share/JetBrains/IntelliJIdea2026.2"
	if runtime.GOOS == "darwin" {
		want = "/home/u/Library/Application Support/JetBrains/IntelliJIdea2026.2/plugins"
	}
	if got != want {
		t.Errorf("PluginsDirFor = %q, want %q", got, want)
	}
}

func TestIntelliJGradlePropertiesKeepsSpacesUnquoted(t *testing.T) {
	ide := IntelliJIDE{
		Home:    "/Applications/IntelliJ IDEA.app",
		Build:   "IU-262.10315.125",
		IdeaVim: "/home/u/My Plugins/IdeaVim",
		JBR:     "/Applications/IntelliJ IDEA.app/Contents/jbr/Contents/Home",
	}
	got := IntelliJGradleProperties(ide)
	for _, want := range []string{
		"platformPath=/Applications/IntelliJ IDEA.app\n",
		"ideaVimPath=/home/u/My Plugins/IdeaVim\n",
		"sinceBuild=262\n",
		"org.gradle.java.installations.paths=/Applications/IntelliJ IDEA.app/Contents/jbr/Contents/Home\n",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("gradle.properties missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, `"`) {
		t.Errorf("paths must not be quoted; Gradle reads the line verbatim:\n%s", got)
	}
}

// An IDE with no build.txt is not an IDE, and one product must be reported
// once however many search directories reach it.
func TestFindIntelliJ(t *testing.T) {
	home := t.TempDir()
	apps := filepath.Join(home, "Applications")
	if runtime.GOOS != "darwin" {
		apps = filepath.Join(home, ".local", "share", "JetBrains", "Toolbox", "apps")
	}
	ide := filepath.Join(apps, "IDEA")
	if runtime.GOOS == "darwin" {
		ide = filepath.Join(apps, "IntelliJ IDEA.app")
	}
	mustWrite(t, buildTxtPath(ide), "IU-262.10315.125\n")
	mustWrite(t, filepath.Join(apps, "NotAnIDE", "readme.txt"), "hi")
	if err := os.MkdirAll(jbrPath(ide), 0o755); err != nil {
		t.Fatal(err)
	}

	found := FindIntelliJ(home)
	if len(found) != 1 {
		t.Fatalf("found %d IDEs, want 1: %+v", len(found), found)
	}
	got := found[0]
	if got.ConfigName != "IntelliJIdea2026.2" {
		t.Errorf("ConfigName = %q", got.ConfigName)
	}
	if got.JBR == "" {
		t.Error("JBR not detected")
	}
	// IdeaVim was never created, and that has to be visible rather than guessed.
	if got.IdeaVim != "" {
		t.Errorf("IdeaVim = %q, want empty", got.IdeaVim)
	}
}

func TestUnzipIntoRejectsEscapingEntry(t *testing.T) {
	dir := t.TempDir()
	zipPath := filepath.Join(dir, "evil.zip")
	f, err := os.Create(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	w, err := zw.Create("../escaped.txt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte("nope")); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	f.Close()

	if _, err := unzipInto(zipPath, filepath.Join(dir, "plugins")); err == nil {
		t.Fatal("unzipInto accepted an entry outside the plugin directory")
	}
}

func TestUnzipIntoReturnsPluginRoot(t *testing.T) {
	dir := t.TempDir()
	zipPath := filepath.Join(dir, "plugin.zip")
	f, err := os.Create(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	w, _ := zw.Create("zmk-vim-mode-intellij/lib/plugin.jar")
	if _, err := w.Write([]byte("jar")); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	f.Close()

	plugins := filepath.Join(dir, "plugins")
	root, err := unzipInto(zipPath, plugins)
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(plugins, "zmk-vim-mode-intellij"); root != want {
		t.Errorf("root = %q, want %q", root, want)
	}
	if _, err := os.Stat(filepath.Join(root, "lib", "plugin.jar")); err != nil {
		t.Errorf("jar not unpacked: %v", err)
	}
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
