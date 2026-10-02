package install

import (
	"archive/zip"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"unicode/utf16"

	intellijplugin "github.com/rafaelromao/zmk-vim-mode/editors/intellij"
)

// IdeaVim releases use different capitalization (including IdeaVIM). Match
// without case sensitivity, but pass the actual directory to the build.
const ideaVimDir = "IdeaVim"

// jetbrainsProducts maps the product code in build.txt to the name JetBrains
// gives that product's configuration directory. Only IDEs that can run
// IdeaVim are listed; Fleet and Toolbox are not among them.
var jetbrainsProducts = map[string]string{
	"IU": "IntelliJIdea", "IC": "IdeaIC",
	"PY": "PyCharm", "PC": "PyCharmCE",
	"WS": "WebStorm", "GO": "GoLand", "RD": "Rider",
	"CL": "CLion", "PS": "PhpStorm", "RM": "RubyMine", "DB": "DataGrip",
}

// IntelliJIDE is one JetBrains IDE found on this machine, with everything the
// build and the install need: where to compile against, where the plugin has
// to land, and which JVM to use.
type IntelliJIDE struct {
	Home       string // the installation, what Gradle builds against
	Build      string // e.g. IU-262.10315.125
	ConfigName string // e.g. IntelliJIdea2026.2
	PluginsDir string // where plugins are unpacked
	IdeaVim    string // the installed IdeaVim, "" when absent
	JBR        string // the bundled JVM, "" when absent
}

// SinceBuild is the branch number the plugin declares compatibility from --
// the "262" of IU-262.10315.125.
func (i IntelliJIDE) SinceBuild() string {
	_, rest, ok := strings.Cut(i.Build, "-")
	if !ok {
		return ""
	}
	branch, _, _ := strings.Cut(rest, ".")
	return branch
}

// ParseBuildTxt turns the contents of an IDE's build.txt into the product code
// and the configuration directory name JetBrains derives from it. The branch
// number carries the version: 262 is 2026.2, 251 is 2025.1.
func ParseBuildTxt(s string) (code, configName string, err error) {
	s = strings.TrimSpace(s)
	code, rest, ok := strings.Cut(s, "-")
	if !ok {
		return "", "", fmt.Errorf("build %q: no product code", s)
	}
	product, known := jetbrainsProducts[code]
	if !known {
		return code, "", fmt.Errorf("build %q: %s is not an IDE that runs IdeaVim", s, code)
	}
	branch, _, _ := strings.Cut(rest, ".")
	if len(branch) != 3 {
		return code, "", fmt.Errorf("build %q: cannot read a version out of %q", s, branch)
	}
	year, err := strconv.Atoi(branch[:2])
	if err != nil {
		return code, "", fmt.Errorf("build %q: %w", s, err)
	}
	return code, fmt.Sprintf("%s%d.%s", product, 2000+year, branch[2:]), nil
}

// jetbrainsConfigRoot is where JetBrains keeps per-IDE configuration, and
// jetbrainsPluginRoot where plugins are unpacked. They are the same directory
// on macOS and different on Linux, which is the kind of detail that makes a
// hand-written path wrong on one of the two.
func jetbrainsConfigRoot(home string) string {
	if runtime.GOOS == "darwin" {
		return filepath.Join(home, "Library", "Application Support", "JetBrains")
	}
	return filepath.Join(home, ".config", "JetBrains")
}

func jetbrainsPluginRoot(home string) string {
	if runtime.GOOS == "darwin" {
		return filepath.Join(home, "Library", "Application Support", "JetBrains")
	}
	return filepath.Join(home, ".local", "share", "JetBrains")
}

// PluginsDirFor returns where a plugin has to be unpacked for the named IDE.
// macOS nests them under the config directory; Linux puts them directly in the
// data directory.
func PluginsDirFor(home, configName string) string {
	if runtime.GOOS == "darwin" {
		return filepath.Join(jetbrainsConfigRoot(home), configName, "plugins")
	}
	return filepath.Join(jetbrainsPluginRoot(home), configName)
}

// intellijSearchDirs lists the places an IDE installation may be found: the
// Toolbox first, since a Toolbox install shadows a standalone one.
func intellijSearchDirs(home string) []string {
	if runtime.GOOS == "darwin" {
		return []string{
			filepath.Join(home, "Applications", "JetBrains Toolbox"),
			filepath.Join(home, "Applications"),
			"/Applications/JetBrains Toolbox",
			"/Applications",
		}
	}
	return []string{
		filepath.Join(home, ".local", "share", "JetBrains", "Toolbox", "apps"),
		"/opt",
		"/usr/share",
		"/usr/local",
		"/var/lib/flatpak/app",
	}
}

// ideHomeCandidates expands one search directory into possible IDE homes. A
// Toolbox app directory holds the installation one or two levels down, so its
// children and grandchildren are both offered; readBuildTxt is what actually
// decides, since only a real IDE home has a build.txt.
func ideHomeCandidates(dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		p := filepath.Join(dir, e.Name())
		out = append(out, p)
		// Toolbox keeps versioned subdirectories under each app.
		if subs, err := os.ReadDir(p); err == nil {
			for _, s := range subs {
				if s.IsDir() {
					out = append(out, filepath.Join(p, s.Name()))
				}
			}
		}
	}
	return out
}

// buildTxtPath is where an IDE records its own build number. On macOS the
// installation is a bundle, so everything sits under Contents.
func buildTxtPath(homeDir string) string {
	if runtime.GOOS == "darwin" && strings.HasSuffix(homeDir, ".app") {
		return filepath.Join(homeDir, "Contents", "Resources", "build.txt")
	}
	return filepath.Join(homeDir, "build.txt")
}

// jbrPath is the JVM the IDE ships, which is the one the build must use: it is
// the JDK the platform expects, and it is already on disk.
func jbrPath(homeDir string) string {
	if runtime.GOOS == "darwin" && strings.HasSuffix(homeDir, ".app") {
		return filepath.Join(homeDir, "Contents", "jbr", "Contents", "Home")
	}
	return filepath.Join(homeDir, "jbr")
}

// FindIntelliJ returns every JetBrains IDE found, newest build first.
func FindIntelliJ(home string) []IntelliJIDE {
	return findIntelliJ(home, intellijSearchDirs(home))
}

func findIntelliJ(home string, searchDirs []string) []IntelliJIDE {
	seen := map[string]bool{}
	var out []IntelliJIDE
	for _, dir := range searchDirs {
		for _, cand := range ideHomeCandidates(dir) {
			raw, err := os.ReadFile(buildTxtPath(cand))
			if err != nil {
				continue
			}
			_, configName, err := ParseBuildTxt(string(raw))
			if err != nil || seen[configName] {
				continue
			}
			seen[configName] = true
			ide := IntelliJIDE{
				Home:       cand,
				Build:      strings.TrimSpace(string(raw)),
				ConfigName: configName,
				PluginsDir: PluginsDirFor(home, configName),
			}
			if p := jbrPath(cand); dirExists(p) {
				ide.JBR = p
			}
			ide.IdeaVim = findIdeaVim(ide.PluginsDir)
			out = append(out, ide)
		}
	}
	sort.Slice(out, func(a, b int) bool { return out[a].Build > out[b].Build })
	return out
}

func dirExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}

func findIdeaVim(pluginsDir string) string {
	entries, err := os.ReadDir(pluginsDir)
	if err != nil {
		return ""
	}
	for _, entry := range entries {
		if strings.EqualFold(entry.Name(), ideaVimDir) {
			p := filepath.Join(pluginsDir, entry.Name())
			if dirExists(p) {
				return p
			}
		}
	}
	return ""
}

// intellijWorkDir is where the plugin is built: a fixed place, so a failed
// build can be inspected and rerun by hand. What is slow to fetch -- the
// Gradle distribution, the Kotlin compiler -- lives in ~/.gradle and survives
// between runs; this directory is emptied before every build (resetWorkDir).
func intellijWorkDir(home string) string {
	if runtime.GOOS == "darwin" {
		return filepath.Join(home, "Library", "Caches", "zmk-vim-mode", "intellij")
	}
	// The XDG spec says to ignore a relative value, and following one would
	// put the directory resetWorkDir empties wherever install was run from.
	if x := os.Getenv("XDG_CACHE_HOME"); filepath.IsAbs(x) {
		return filepath.Join(x, "zmk-vim-mode", "intellij")
	}
	return filepath.Join(home, ".cache", "zmk-vim-mode", "intellij")
}

// resetWorkDir empties the build directory and recreates it. Gradle runs
// whatever build logic it finds in a project -- a buildSrc/, a settings.gradle
// next to ours -- and the installer installs the last zip it finds in
// build/distributions, so nothing an earlier run or anyone else left there may
// survive into this build.
func resetWorkDir(dir string) error {
	if err := os.RemoveAll(dir); err != nil {
		return err
	}
	return os.MkdirAll(dir, 0o700)
}

// materializeIntelliJ writes the embedded sources into dir, along with a
// gradle.properties describing the IDE we are building for.
func materializeIntelliJ(dir string, ide IntelliJIDE) error {
	err := fs.WalkDir(intellijplugin.Files, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || p == "." {
			return err
		}
		target := filepath.Join(dir, filepath.FromSlash(p))
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		b, err := intellijplugin.Files.ReadFile(p)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		mode := os.FileMode(0o644)
		if p == "gradlew" {
			mode = 0o755
		}
		if old, err := os.ReadFile(target); err == nil && string(old) == string(b) {
			return nil
		}
		return os.WriteFile(target, b, mode)
	})
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "gradle.properties"),
		[]byte(IntelliJGradleProperties(ide)), 0o644)
}

// IntelliJGradleProperties is the generated build configuration for one IDE.
// Paths may contain spaces and must not be quoted: a value runs to the end of
// its line. The values come from directory names and build.txt, so they are
// escaped (propValue) -- a line break in one would otherwise start a property
// of its own.
func IntelliJGradleProperties(ide IntelliJIDE) string {
	var b strings.Builder
	b.WriteString("# Generated by `zmk-vim-mode install --intellij`. Edit and it will be overwritten.\n")
	fmt.Fprintf(&b, "platformPath=%s\n", propValue(ide.Home))
	fmt.Fprintf(&b, "ideaVimPath=%s\n", propValue(ide.IdeaVim))
	fmt.Fprintf(&b, "sinceBuild=%s\n", propValue(ide.SinceBuild()))
	if ide.JBR != "" {
		fmt.Fprintf(&b, "org.gradle.java.installations.paths=%s\n", propValue(ide.JBR))
	}
	b.WriteString("kotlin.stdlib.default.dependency=false\n")
	b.WriteString("org.gradle.jvmargs=-Xmx2g\n")
	return b.String()
}

// propValue escapes s as a java.util.Properties value. A backslash starts an
// escape and a line break ends the value, so both are escaped, as are the
// other control characters; anything outside ASCII becomes \uXXXX, which reads
// the same whichever encoding the file is opened with.
func propValue(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r == '\\':
			b.WriteString(`\\`)
		case r == '\n':
			b.WriteString(`\n`)
		case r == '\r':
			b.WriteString(`\r`)
		case r == '\t':
			b.WriteString(`\t`)
		case r == '\f':
			b.WriteString(`\f`)
		case r < 0x20 || r > 0x7e:
			for _, u := range utf16.Encode([]rune{r}) {
				fmt.Fprintf(&b, `\u%04x`, u)
			}
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// buildIntelliJPlugin runs the Gradle wrapper and returns the distribution zip.
func buildIntelliJPlugin(w io.Writer, dir string, ide IntelliJIDE) (string, error) {
	cmd := exec.Command(filepath.Join(dir, "gradlew"), "--quiet", "buildPlugin")
	cmd.Dir = dir
	cmd.Env = os.Environ()
	if ide.JBR != "" {
		// The wrapper needs a JVM before Gradle can pick a toolchain, and the
		// IDE's own is the one we know exists and matches.
		cmd.Env = append(cmd.Env, "JAVA_HOME="+ide.JBR)
	}
	cmd.Stdout, cmd.Stderr = w, w
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("gradlew buildPlugin: %w", err)
	}
	zips, err := filepath.Glob(filepath.Join(dir, "build", "distributions", "*.zip"))
	if err != nil || len(zips) == 0 {
		return "", fmt.Errorf("the build produced no zip in %s", filepath.Join(dir, "build", "distributions"))
	}
	sort.Strings(zips)
	return zips[len(zips)-1], nil
}

// unzipInto unpacks a plugin distribution into the IDE's plugin directory,
// which is exactly what "Install Plugin from Disk" does. The zip's own top
// directory is kept, so the plugin replaces its previous version rather than
// scattering files.
func unzipInto(zipPath, pluginsDir string) (string, error) {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return "", err
	}
	defer r.Close()
	root := ""
	for _, f := range r.File {
		name := filepath.Clean(filepath.FromSlash(f.Name))
		// A zip may name any path it likes; refuse to write outside the target.
		if name == ".." || strings.HasPrefix(name, ".."+string(os.PathSeparator)) || filepath.IsAbs(name) {
			return "", fmt.Errorf("%s: entry %q escapes the plugin directory", zipPath, f.Name)
		}
		if root == "" {
			root = strings.Split(name, string(os.PathSeparator))[0]
		}
		target := filepath.Join(pluginsDir, name)
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return "", err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return "", err
		}
		if err := copyZipEntry(f, target); err != nil {
			return "", err
		}
	}
	if root == "" {
		return "", fmt.Errorf("%s: empty zip", zipPath)
	}
	return filepath.Join(pluginsDir, root), nil
}

func copyZipEntry(f *zip.File, target string) error {
	src, err := f.Open()
	if err != nil {
		return err
	}
	defer src.Close()
	dst, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer dst.Close()
	_, err = io.Copy(dst, src)
	return err
}

// intellijRunning reports whether a JetBrains IDE is running. Plugins are
// scanned at startup, so one that is running has to be restarted before it
// sees the install.
func intellijRunning() bool {
	// Matching a bare "idea" would catch any command line that happens to
	// contain the word; every real install path carries the vendor name.
	out, err := exec.Command("pgrep", "-f", "JetBrains|jetbrains").Output()
	return err == nil && len(strings.TrimSpace(string(out))) > 0
}

// InstallIntelliJ finds every JetBrains IDE with IdeaVim, builds the plugin
// against each and unpacks it where that IDE will find it.
func InstallIntelliJ(w io.Writer) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	ides := FindIntelliJ(home)
	if len(ides) == 0 {
		fmt.Fprintln(w, "intellij   : no JetBrains IDE found; nothing to do")
		return nil
	}
	installed := 0
	for _, ide := range ides {
		if ide.IdeaVim == "" {
			fmt.Fprintf(w, "intellij   : %s — IdeaVim is not installed; add it (Settings → Plugins → Marketplace) and run this again\n",
				ide.ConfigName)
			continue
		}
		if ide.JBR == "" {
			fmt.Fprintf(w, "intellij   : %s — no bundled JVM at %s; cannot build\n",
				ide.ConfigName, TrimHome(jbrPath(ide.Home)))
			continue
		}
		dir := intellijWorkDir(home)
		if err := resetWorkDir(dir); err != nil {
			return err
		}
		if err := materializeIntelliJ(dir, ide); err != nil {
			return fmt.Errorf("%s: %w", ide.ConfigName, err)
		}
		fmt.Fprintf(w, "intellij   : %s (%s) — building, first run downloads Gradle and the Kotlin compiler\n",
			ide.ConfigName, ide.Build)
		zipPath, err := buildIntelliJPlugin(w, dir, ide)
		if err != nil {
			fmt.Fprintf(w, "intellij   : %s — build failed: %v\n", ide.ConfigName, err)
			fmt.Fprintf(w, "             build it yourself in %s, then use Settings → Plugins → ⚙ → Install Plugin from Disk\n", TrimHome(dir))
			continue
		}
		dest, err := unzipInto(zipPath, ide.PluginsDir)
		if err != nil {
			return fmt.Errorf("%s: %w", ide.ConfigName, err)
		}
		fmt.Fprintf(w, "intellij   : %s — installed into %s\n", ide.ConfigName, TrimHome(dest))
		installed++
	}
	if installed > 0 {
		if intellijRunning() {
			fmt.Fprintln(w, "an IDE is running, and plugins are only scanned at startup: restart it.")
		}
		fmt.Fprintln(w, "then open a project; `zmk-vim-mode status` should list an `intellij` client.")
	}
	return nil
}
