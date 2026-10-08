package junit

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReportPublishesOnlyCompleteXML(t *testing.T) {
	for _, tc := range []struct {
		text  string
		valid bool
	}{
		{`<?xml version="1.0"?><testsuites><testsuite><testcase name="ok"/></testsuite></testsuites>`, true},
		{`<testsuite><testcase><failure>failed</failure></testcase></testsuite>`, true},
		{`<testsuites>`, false}, {`<testsuites></testsuite>`, false},
		{`<testsuites/><testsuites/>`, false}, {`<not-junit/>`, false}, {`text<testsuites/>`, false},
	} {
		dir := t.TempDir()
		partial, final := filepath.Join(dir, "partial"), filepath.Join(dir, "test.xml")
		if err := os.WriteFile(partial, []byte(tc.text), 0o600); err != nil {
			t.Fatal(err)
		}
		err := Publish(partial, final)
		if (err == nil) != tc.valid {
			t.Fatalf("%s: %v", tc.text, err)
		}
		data, err := os.ReadFile(final)
		if tc.valid && (err != nil || string(data) != tc.text) || !tc.valid && !os.IsNotExist(err) {
			t.Fatalf("published %q: %v", data, err)
		}
	}
}
