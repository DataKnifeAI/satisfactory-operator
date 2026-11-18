/*
Copyright 2026 DataKnifeAI.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package config

import (
	"regexp"
)

var (
	saveGuidRE      = regexp.MustCompile(`Starting save \(Guid\[([0-9A-Fa-f]{32})\]`)
	joinSucceededRE = regexp.MustCompile(`LogNet: Join succeeded:`)
	buildIDRE       = regexp.MustCompile(`Setting \["BuildId"\] written with key\[xb\] value\[(\d+)\]`)
)

// ParseWorldSaveGuid extracts the world save GUID from server log text.
func ParseWorldSaveGuid(logText string) string {
	if m := saveGuidRE.FindStringSubmatch(logText); len(m) == 2 {
		return m[1]
	}
	return ""
}

// ParseBuildID extracts the session BuildId from server log text.
func ParseBuildID(logText string) string {
	if m := buildIDRE.FindStringSubmatch(logText); len(m) == 2 {
		return m[1]
	}
	return ""
}

// HasSuccessfulJoin reports whether a player join succeeded in the log sample.
func HasSuccessfulJoin(logText string) bool {
	return joinSucceededRE.MatchString(logText)
}
