package action

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStampEnvSubstitutesStableKeys(t *testing.T) {
	status := filepath.Join(t.TempDir(), "stable-status.txt")
	if err := os.WriteFile(status, []byte("BUILD_EMBED_LABEL \nSTABLE_RELEASE_VERSION 0.0.0-20260924051700+g0123456789ab\r\nSTABLE_GIT_COMMIT abc\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	env, err := stampEnv(status, map[string]string{
		"SITE_VERSION": "{STABLE_RELEASE_VERSION}",
		"LABEL":        "site {STABLE_GIT_COMMIT}",
		"PLAIN":        "{NOT_A_STATUS_KEY}",
	})
	if err != nil {
		t.Fatal(err)
	}
	if env["SITE_VERSION"] != "0.0.0-20260924051700+g0123456789ab" || env["LABEL"] != "site abc" || env["PLAIN"] != "{NOT_A_STATUS_KEY}" {
		t.Fatalf("env = %v", env)
	}
}

func TestStampEnvRefusesAMissingKey(t *testing.T) {
	status := filepath.Join(t.TempDir(), "stable-status.txt")
	if err := os.WriteFile(status, []byte("BUILD_EMBED_LABEL \n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := stampEnv(status, map[string]string{"SITE_VERSION": "{STABLE_RELEASE_VERSION}"})
	if err == nil || !strings.Contains(err.Error(), "STABLE_RELEASE_VERSION") {
		t.Fatalf("err = %v", err)
	}
}
