package testenv

import (
	"errors"
	"math"
	"os"
	"strconv"
	"strings"
	"unicode"
)

type Environment struct {
	XML, Coverage, Outputs, Filter string
	Shard, Total                   int64
}

func Read(lookup func(string) (string, bool)) (Environment, error) {
	get := func(key string) string { value, _ := lookup(key); return value }
	env := Environment{XML: get("XML_OUTPUT_FILE"), Coverage: get("COVERAGE_DIR"), Outputs: get("TEST_UNDECLARED_OUTPUTS_DIR"), Filter: get("TESTBRIDGE_TEST_ONLY")}
	if env.Outputs != "" {
		if err := os.MkdirAll(env.Outputs, 0o755); err != nil {
			return env, err
		}
	}
	if total := get("TEST_TOTAL_SHARDS"); total != "" {
		number := func(value string) (int64, bool) {
			// Number() accepts ECMAScript whitespace and radix literals, but not
			// Go's numeric separators or hexadecimal floating-point syntax.
			value = strings.TrimFunc(value, func(r rune) bool {
				return strings.ContainsRune("\t\n\v\f\r\u2028\u2029\ufeff", r) || unicode.Is(unicode.Zs, r)
			})
			if value == "" {
				return 0, true
			}
			if strings.Contains(value, "_") {
				return 0, false
			}
			if len(value) > 2 && value[0] == '0' && strings.ContainsAny(value[1:2], "xXoObB") {
				n, err := strconv.ParseUint(value, 0, 64)
				return int64(n), err == nil && n <= 9007199254740991
			}
			if strings.ContainsAny(value, "pPxX") {
				return 0, false
			}
			n, err := strconv.ParseFloat(value, 64)
			return int64(n), err == nil && !math.IsNaN(n) && !math.IsInf(n, 0) && math.Trunc(n) == n && math.Abs(n) <= 9007199254740991
		}
		count, countOK := number(total)
		rawIndex, present := lookup("TEST_SHARD_INDEX")
		index, indexOK := number(rawIndex)
		indexOK = indexOK && present
		if !countOK || !indexOK || count < 1 || index < 0 || index >= count {
			return env, errors.New("invalid Bazel test shard index or count")
		}
		env.Shard, env.Total = index+1, count
		if status := get("TEST_SHARD_STATUS_FILE"); status != "" {
			if err := os.WriteFile(status, nil, 0o644); err != nil {
				return env, err
			}
		}
	}
	return env, nil
}
