package testenv

import (
	"os"
	"path/filepath"
	"testing"
)

func TestShardParsing(t *testing.T) {
	for _, tc := range []struct {
		total, index   string
		present, valid bool
		shard          int64
	}{
		{"3", "1", true, true, 2}, {"3", "", true, true, 1}, {"3", "", false, false, 0}, {"3", "-1", true, false, 0},
		{"3", "3", true, false, 0}, {"3", "1.5", true, false, 0}, {"3", "NaN", true, false, 0},
		{"Infinity", "0", true, false, 0}, {"9007199254740992", "0", true, false, 0}, {"0", "0", true, false, 0},
		{"0x3", "0b1", true, true, 2}, {"1_0", "0", true, false, 0}, {"0x1_0", "0", true, false, 0}, {"+0x1p2", "0", true, false, 0}, {"\ufeff3\ufeff", "0", true, true, 1}, {"\u00853", "0", true, false, 0}, {"3e0", "1e0", true, true, 2}, {"", "nonsense", true, true, 0},
	} {
		t.Run(tc.total+"/"+tc.index+"/"+string(rune('0'+tc.shard)), func(t *testing.T) {
			status := filepath.Join(t.TempDir(), "sharded")
			values := map[string]string{"TEST_TOTAL_SHARDS": tc.total, "TEST_SHARD_STATUS_FILE": status}
			if tc.present {
				values["TEST_SHARD_INDEX"] = tc.index
			}
			env, err := Read(func(key string) (string, bool) { v, ok := values[key]; return v, ok })
			if (err == nil) != tc.valid || err == nil && env.Shard != tc.shard {
				t.Fatalf("env=%+v err=%v", env, err)
			}
			_, err = os.Stat(status)
			if (err == nil) != (tc.valid && tc.total != "") {
				t.Fatalf("shard status: %v", err)
			}
		})
	}
}

func TestEmptyValues(t *testing.T) {
	env, err := Read(func(string) (string, bool) { return "", true })
	if err != nil || env != (Environment{}) {
		t.Fatalf("empty values: %+v %v", env, err)
	}
}
