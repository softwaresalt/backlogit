package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/softwaresalt/backlogit/internal/release"
	"github.com/softwaresalt/backlogit/internal/version"
)

const (
	updateCheckEnvVar  = "BACKLOGIT_NO_UPDATE_CHECK"
	updateCheckTimeout = 1 * time.Second
)

type latestVersionLookupFunc func(context.Context) (string, error)

type versionLatestLookupContextKey struct{}

func defaultVersionLatestLookup(ctx context.Context) (string, error) {
	client := release.Client{
		Token: os.Getenv("GITHUB_TOKEN"),
	}
	return client.Latest(ctx)
}

// versionInfo holds the structured version payload for JSON output.
type versionInfo struct {
	Version         string `json:"version"`
	Current         string `json:"current"`
	Latest          string `json:"latest"`
	UpdateAvailable bool   `json:"update_available"`
	UpdateCheck     string `json:"update_check"`
	Commit          string `json:"commit"`
	BuildDate       string `json:"build_date"`
	GoVersion       string `json:"go_version"`
}

// newVersionCommandWithLookup creates the `backlogit version` subcommand.
// It prints version, commit, build date, and Go runtime version.
// --format=json emits a structured JSON object.
func newVersionCommandWithLookup(lookup latestVersionLookupFunc) *cobra.Command {
	var noUpdateCheck bool
	if lookup == nil {
		lookup = defaultVersionLatestLookup
	}
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Print version, latest release, commit, build date, and Go runtime information",
		Long: `Print build metadata and latest-release status.

By default, backlogit performs a bounded latest-release check against GitHub.
Use --no-update-check or set BACKLOGIT_NO_UPDATE_CHECK to one of
1, true, t, yes, y, or on to skip the remote call for CI and scripts.`,
		Example: `  backlogit version
  backlogit version --no-update-check
  backlogit version --format json`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			format, err := cmd.Flags().GetString("format")
			if err != nil {
				return fmt.Errorf("read format flag: %w", err)
			}
			if format != "" && format != "json" {
				return fmt.Errorf("unsupported format %q: version supports: json", format)
			}
			inheritedNoUpdateCheck, err := noUpdateCheckFlagValue(cmd)
			if err != nil {
				return err
			}
			info := collectVersionInfo(cmd, noUpdateCheck || inheritedNoUpdateCheck, lookup)
			if format == "json" {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				if err := enc.Encode(info); err != nil {
					return fmt.Errorf("write version JSON: %w", err)
				}
				return nil
			}
			buildDate := info.BuildDate
			if buildDate == "" {
				buildDate = "unknown"
			}
			if _, err := fmt.Fprintf(cmd.OutOrStdout(), "version    %s\nlatest     %s\ncommit     %s\nbuild date %s\ngo version %s\n",
				info.Current, formatLatestLine(info), info.Commit, buildDate, info.GoVersion); err != nil {
				return fmt.Errorf("write version output: %w", err)
			}
			return nil
		},
	}
	cmd.Flags().String("format", "", "output format: json (default: human)")
	cmd.Flags().BoolVar(&noUpdateCheck, "no-update-check", false, "skip the remote latest-release check")
	return cmd
}

func collectVersionInfo(cmd *cobra.Command, noUpdateCheck bool, lookup latestVersionLookupFunc) versionInfo {
	current := version.Resolve()
	info := versionInfo{
		Version:     current,
		Current:     current,
		UpdateCheck: "unavailable",
		Commit:      version.Commit,
		BuildDate:   version.BuildDate,
		GoVersion:   runtime.Version(),
	}
	if noUpdateCheck || envBool(updateCheckEnvVar) {
		info.UpdateCheck = "skipped"
		return info
	}
	if lookup == nil {
		lookup = defaultVersionLatestLookup
	}

	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, updateCheckTimeout)
	defer cancel()

	latest, err := lookup(ctx)
	if err != nil {
		return info
	}
	info.Latest = latest
	info.UpdateAvailable, info.UpdateCheck = release.UpdateAvailability(current, latest)
	return info
}

func formatLatestLine(info versionInfo) string {
	switch info.UpdateCheck {
	case "ok":
		if info.UpdateAvailable {
			return fmt.Sprintf("%s (update available; run 'backlogit update')", info.Latest)
		}
		return fmt.Sprintf("%s (up to date)", info.Latest)
	case "uncomparable":
		return fmt.Sprintf("%s (update status unavailable for current version)", info.Latest)
	case "skipped":
		return "skipped (update check skipped)"
	default:
		return "unavailable (update check unavailable)"
	}
}

func formatRootVersionLine(cmd *cobra.Command) string {
	noUpdateCheck, err := noUpdateCheckFlagValue(cmd)
	if err != nil {
		noUpdateCheck = true
	}
	info := collectVersionInfo(cmd, noUpdateCheck, latestLookupFromCommand(cmd))
	base := fmt.Sprintf("backlogit version %s", info.Current)
	switch info.UpdateCheck {
	case "ok":
		if info.UpdateAvailable {
			return fmt.Sprintf("%s (latest: %s -- update available; run 'backlogit update')", base, info.Latest)
		}
		return fmt.Sprintf("%s (latest: %s -- up to date)", base, info.Latest)
	case "uncomparable":
		return fmt.Sprintf("%s (latest: %s -- update status unavailable for current version)", base, info.Latest)
	case "skipped":
		return base + " (update check skipped)"
	default:
		return base + " (update check unavailable)"
	}
}

func latestLookupFromCommand(cmd *cobra.Command) latestVersionLookupFunc {
	if cmd != nil {
		if lookup, ok := cmd.Context().Value(versionLatestLookupContextKey{}).(latestVersionLookupFunc); ok && lookup != nil {
			return lookup
		}
	}
	return defaultVersionLatestLookup
}

func noUpdateCheckFlagValue(cmd *cobra.Command) (bool, error) {
	for c := cmd; c != nil; c = c.Parent() {
		if flag := c.Flags().Lookup("no-update-check"); flag != nil && flag.Changed {
			value, err := strconv.ParseBool(flag.Value.String())
			if err != nil {
				return false, fmt.Errorf("parse no-update-check flag: %w", err)
			}
			return value, nil
		}
		if flag := c.PersistentFlags().Lookup("no-update-check"); flag != nil && flag.Changed {
			value, err := strconv.ParseBool(flag.Value.String())
			if err != nil {
				return false, fmt.Errorf("parse no-update-check flag: %w", err)
			}
			return value, nil
		}
	}
	return false, nil
}

func envBool(name string) bool {
	value := strings.TrimSpace(strings.ToLower(os.Getenv(name)))
	switch value {
	case "1", "true", "t", "yes", "y", "on":
		return true
	default:
		return false
	}
}
