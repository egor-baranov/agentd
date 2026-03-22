package runtime

import (
	"fmt"
	"sort"
	"strings"
)

func ParseNodeEndpoints(spec string) (map[string]string, error) {
	out := map[string]string{}
	if spec == "" {
		return out, nil
	}
	for _, part := range strings.Split(spec, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		pieces := strings.SplitN(part, "=", 2)
		if len(pieces) != 2 {
			return nil, fmt.Errorf("invalid node endpoint %q, want id=addr", part)
		}
		out[strings.TrimSpace(pieces[0])] = strings.TrimSpace(pieces[1])
	}
	return out, nil
}

func NodeIDs(endpoints map[string]string) []string {
	out := make([]string, 0, len(endpoints))
	for id := range endpoints {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}
