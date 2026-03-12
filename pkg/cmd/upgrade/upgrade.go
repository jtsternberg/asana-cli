package upgrade

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"
	"github.com/timwehrle/asana/internal/build"
	"github.com/timwehrle/asana/internal/prompter"
	"github.com/timwehrle/asana/pkg/factory"
	"github.com/timwehrle/asana/pkg/iostreams"
)

const (
	// githubOwner and githubRepo identify the release repository.
	// Note: the Go module is github.com/timwehrle/asana but the releases live
	// in the jtsternberg/asana-cli repository (the fork that ships binaries).
	githubOwner = "jtsternberg"
	githubRepo  = "asana-cli"
	apiURL      = "https://api.github.com/repos/" + githubOwner + "/" + githubRepo + "/releases/latest"
)

// UpgradeOptions holds all options for the upgrade command.
type UpgradeOptions struct {
	IO       *iostreams.IOStreams
	Prompter prompter.Prompter
	Yes      bool
}

type githubRelease struct {
	TagName string `json:"tag_name"`
	HTMLURL string `json:"html_url"`
	Assets  []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

// NewCmdUpgrade creates the upgrade cobra command.
func NewCmdUpgrade(f factory.Factory, runF func(*UpgradeOptions) error) *cobra.Command {
	opts := &UpgradeOptions{
		IO:       f.IOStreams,
		Prompter: f.Prompter,
	}

	cmd := &cobra.Command{
		Use:   "upgrade",
		Short: "Upgrade the asana CLI to the latest version",
		Long: heredoc.Doc(`
			Upgrade the asana CLI to the latest version.

			The command detects whether you installed via git (source) or as a
			pre-built binary, and updates accordingly.

			For git installs the source tree is updated with "git pull" and the
			binary is rebuilt with "go install ./cmd/asana".

			For binary installs the latest release is downloaded from GitHub and
			replaces the currently running binary.
		`),
		Example: heredoc.Docf(`
			# Upgrade to the latest version
			$ %[1]s upgrade

			# Upgrade without confirmation prompt
			$ %[1]s upgrade --yes
		`, "asana"),
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if runF != nil {
				return runF(opts)
			}
			return runUpgrade(opts)
		},
	}

	cmd.Flags().BoolVarP(&opts.Yes, "yes", "y", false, "Skip confirmation prompt")

	return cmd
}

func runUpgrade(opts *UpgradeOptions) error {
	io := opts.IO
	cs := io.ColorScheme()

	fmt.Fprintf(io.Out, "Current version: %s\n\n", cs.Bold(build.Version))

	if sourceDir, ok := detectSourceInstall(); ok {
		return upgradeFromGit(opts, sourceDir)
	}

	return upgradeFromBinary(opts)
}

// detectSourceInstall walks up from the current working directory looking for
// the asana source root (a directory that contains both a .git folder and a
// go.mod that declares the timwehrle/asana module). Returns the root path and
// true when found.
func detectSourceInstall() (string, bool) {
	dir, err := os.Getwd()
	if err != nil {
		return "", false
	}

	for {
		if isAsanaSourceDir(dir) {
			return dir, true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return "", false
}

func isAsanaSourceDir(dir string) bool {
	if _, err := os.Stat(filepath.Join(dir, ".git")); err != nil {
		return false
	}
	data, err := os.ReadFile(filepath.Join(dir, "go.mod"))
	if err != nil {
		return false
	}
	return strings.Contains(string(data), "github.com/timwehrle/asana")
}

// upgradeFromGit updates the CLI from a local git source clone.
func upgradeFromGit(opts *UpgradeOptions, sourceDir string) error {
	io := opts.IO
	cs := io.ColorScheme()

	fmt.Fprintf(io.Out, "Install method: %s\n\n", cs.Bold("git source"))

	// Check for uncommitted changes.
	statusOut, err := runGitCommand(sourceDir, "status", "--porcelain")
	if err != nil {
		return fmt.Errorf("failed to check git status: %w", err)
	}
	if strings.TrimSpace(statusOut) != "" {
		fmt.Fprintf(io.Out, "%s Uncommitted changes detected in %s.\n", cs.WarningIcon, sourceDir)
		fmt.Fprintf(io.Out, "  Stash or commit your changes before upgrading.\n\n")
		return errors.New("upgrade aborted: working tree is dirty")
	}

	// Check current branch.
	branch, err := runGitCommand(sourceDir, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return fmt.Errorf("failed to determine git branch: %w", err)
	}
	branch = strings.TrimSpace(branch)
	if branch != "main" && branch != "master" {
		fmt.Fprintf(io.Out, "%s Not on main/master branch (currently on %s).\n", cs.WarningIcon, branch)
	}

	// Confirm upgrade.
	if !opts.Yes {
		confirmed, err := confirmPrompt(opts, fmt.Sprintf("Pull latest changes and rebuild from %s?", sourceDir))
		if err != nil {
			return err
		}
		if !confirmed {
			fmt.Fprintln(io.Out, "Upgrade cancelled.")
			return nil
		}
	}

	// Record HEAD before pulling so we can show a changelog.
	prevHead, err := runGitCommand(sourceDir, "rev-parse", "HEAD")
	if err != nil {
		return fmt.Errorf("failed to get current HEAD: %w", err)
	}
	prevHead = strings.TrimSpace(prevHead)

	// git pull
	fmt.Fprintln(io.Out, "Pulling latest changes...")
	if err := streamCommand(io.Out, sourceDir, "git", "pull"); err != nil {
		return fmt.Errorf("git pull failed: %w", err)
	}

	// go install
	fmt.Fprintln(io.Out, "\nRebuilding and installing...")
	if err := streamCommand(io.Out, sourceDir, "go", "install", "./cmd/asana"); err != nil {
		return fmt.Errorf("go install failed: %w", err)
	}

	// Show changelog (new commits since prevHead).
	newHead, err := runGitCommand(sourceDir, "rev-parse", "HEAD")
	if err == nil {
		newHead = strings.TrimSpace(newHead)
		if newHead != prevHead {
			fmt.Fprintln(io.Out, "\nChangelog:")
			log, logErr := runGitCommand(sourceDir, "log", "--oneline", prevHead+".."+newHead)
			if logErr == nil && strings.TrimSpace(log) != "" {
				for _, line := range strings.Split(strings.TrimSpace(log), "\n") {
					fmt.Fprintf(io.Out, "  %s\n", line)
				}
			}
		}
	}

	fmt.Fprintln(io.Out)
	return runHealthCheck(opts)
}

// upgradeFromBinary downloads the latest pre-built release from GitHub and
// replaces the currently running binary.
func upgradeFromBinary(opts *UpgradeOptions) error {
	io := opts.IO
	cs := io.ColorScheme()

	fmt.Fprintf(io.Out, "Install method: %s\n\n", cs.Bold("pre-built binary"))

	// Fetch latest release info.
	fmt.Fprintln(io.Out, "Fetching latest release information...")
	release, err := fetchLatestRelease()
	if err != nil {
		return fmt.Errorf("failed to fetch release info: %w", err)
	}

	latestVersion := strings.TrimPrefix(release.TagName, "v")
	currentVersion := strings.TrimPrefix(build.Version, "v")

	fmt.Fprintf(io.Out, "Latest version:  %s\n", cs.Bold(release.TagName))

	if currentVersion != "dev" && currentVersion == latestVersion {
		fmt.Fprintf(io.Out, "\n%s Already up to date!\n", cs.SuccessIcon)
		return nil
	}

	// Determine asset name for current platform.
	assetName, err := platformAssetName()
	if err != nil {
		return err
	}

	// Find matching asset.
	downloadURL := ""
	for _, a := range release.Assets {
		if a.Name == assetName {
			downloadURL = a.BrowserDownloadURL
			break
		}
	}
	if downloadURL == "" {
		return fmt.Errorf("no release asset found for %s – check %s", assetName, release.HTMLURL)
	}

	// Confirm upgrade.
	if !opts.Yes {
		confirmed, err := confirmPrompt(opts, fmt.Sprintf("Upgrade from %s to %s?", build.Version, release.TagName))
		if err != nil {
			return err
		}
		if !confirmed {
			fmt.Fprintln(io.Out, "Upgrade cancelled.")
			return nil
		}
	}

	// Find current executable path.
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("could not determine executable path: %w", err)
	}
	exePath, err = filepath.EvalSymlinks(exePath)
	if err != nil {
		return fmt.Errorf("could not resolve executable symlinks: %w", err)
	}

	// Download tarball to a temp directory.
	tmpDir, err := os.MkdirTemp("", "asana-upgrade-*")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	tarballPath := filepath.Join(tmpDir, assetName)
	fmt.Fprintf(io.Out, "\nDownloading %s...\n", assetName)
	if err := downloadFile(tarballPath, downloadURL); err != nil {
		return fmt.Errorf("download failed: %w", err)
	}

	// Extract binary from tarball.
	newBinaryPath := filepath.Join(tmpDir, "asana")
	if err := extractBinary(tarballPath, newBinaryPath); err != nil {
		return fmt.Errorf("failed to extract binary: %w", err)
	}

	// Replace current binary.
	fmt.Fprintf(io.Out, "Installing to %s...\n", exePath)
	if err := replaceBinary(exePath, newBinaryPath); err != nil {
		return fmt.Errorf("failed to install new binary: %w", err)
	}

	fmt.Fprintf(io.Out, "\n%s Upgraded to %s\n", cs.SuccessIcon, cs.Bold(release.TagName))
	fmt.Fprintf(io.Out, "\nRelease notes: %s\n\n", release.HTMLURL)

	return runHealthCheck(opts)
}

// fetchLatestRelease queries the GitHub API for the latest release.
func fetchLatestRelease() (*githubRelease, error) {
	req, err := http.NewRequest(http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned HTTP %d", resp.StatusCode)
	}

	var release githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, err
	}

	return &release, nil
}

// platformAssetName returns the expected asset filename for the current OS/arch.
func platformAssetName() (string, error) {
	goos := runtime.GOOS
	switch goos {
	case "linux":
		goos = "Linux"
	case "darwin":
		goos = "Darwin"
	case "windows":
		goos = "Windows"
	default:
		return "", fmt.Errorf("unsupported OS: %s", goos)
	}

	arch := runtime.GOARCH
	switch arch {
	case "amd64":
		arch = "x86_64"
	case "arm64":
		arch = "arm64"
	case "arm":
		arch = "armv7"
	case "386":
		arch = "i386"
	default:
		return "", fmt.Errorf("unsupported architecture: %s", arch)
	}

	return fmt.Sprintf("asana_%s_%s.tar.gz", goos, arch), nil
}

// downloadFile downloads url to the given local path.
func downloadFile(dest, url string) error {
	resp, err := http.Get(url) //nolint:gosec // URL is from the official GitHub API response
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d while downloading %s", resp.StatusCode, url)
	}

	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = io.Copy(f, resp.Body)
	return err
}

// extractBinary extracts the "asana" binary from a .tar.gz archive.
func extractBinary(tarballPath, destPath string) error {
	f, err := os.Open(tarballPath)
	if err != nil {
		return err
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return err
		}

		// Look for the binary (top-level file named "asana" or "asana.exe").
		base := filepath.Base(hdr.Name)
		if hdr.Typeflag == tar.TypeReg && (base == "asana" || base == "asana.exe") {
			return writeBinary(destPath, tr)
		}
	}

	return fmt.Errorf("asana binary not found in archive")
}

// writeBinary writes the binary from r to destPath with executable permissions.
func writeBinary(destPath string, r io.Reader) error {
	out, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, r) //nolint:gosec // source is a known release tarball
	return err
}

// replaceBinary replaces the target binary with the new one. It first tries a
// direct rename; if that fails (e.g. cross-device), it copies the file instead.
func replaceBinary(targetPath, newBinaryPath string) error {
	// Try atomic rename first.
	if err := os.Rename(newBinaryPath, targetPath); err == nil {
		return nil
	}

	// Fall back to copy + replace.
	src, err := os.Open(newBinaryPath)
	if err != nil {
		return err
	}
	defer src.Close()

	tmpTarget := targetPath + ".upgrade-tmp"
	dst, err := os.OpenFile(tmpTarget, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return fmt.Errorf("cannot write to %s (try running with sudo): %w", targetPath, err)
	}
	defer func() {
		dst.Close()
		os.Remove(tmpTarget)
	}()

	if _, err := io.Copy(dst, src); err != nil {
		return err
	}
	dst.Close()

	return os.Rename(tmpTarget, targetPath)
}

// runHealthCheck executes the newly installed binary with --version to verify
// it is working.
func runHealthCheck(opts *UpgradeOptions) error {
	io := opts.IO
	cs := io.ColorScheme()

	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("could not locate binary for health check: %w", err)
	}

	out, err := exec.Command(exePath, "--version").Output()
	if err != nil {
		fmt.Fprintf(io.Out, "%s Health check failed: %v\n", cs.ErrorIcon, err)
		return nil
	}

	version := strings.TrimSpace(string(out))
	fmt.Fprintf(io.Out, "%s Health check passed: %s\n", cs.SuccessIcon, version)
	return nil
}

// confirmPrompt asks the user for yes/no confirmation.
func confirmPrompt(opts *UpgradeOptions, message string) (bool, error) {
	if opts.Prompter == nil {
		return false, errors.New("no prompter available; use --yes to skip confirmation")
	}
	return opts.Prompter.Confirm(message, "Yes")
}

// runGitCommand runs a git subcommand in dir and returns its stdout.
func runGitCommand(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// streamCommand runs a command in dir, streaming its output to w.
func streamCommand(w io.Writer, dir string, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Stdout = w
	cmd.Stderr = w
	return cmd.Run()
}
