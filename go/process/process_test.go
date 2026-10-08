package process

import (
	"reflect"
	"testing"
)

func TestEnvironmentCaseAndEmptyValues(t *testing.T) {
	got := environment([]string{"PATH=old", "node_v8_coverage=raw", "KEEP="}, map[string]string{"Path": "new"}, "NODE_V8_COVERAGE")
	if !reflect.DeepEqual(got, []string{"KEEP=", "Path=new"}) {
		t.Fatal(got)
	}

}
