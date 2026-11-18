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
	"strings"
	"testing"
)

func TestRenderSettingsEnv(t *testing.T) {
	got := RenderSettingsEnv(ServerSettings{
		ServerName:   "Factory",
		MaxPlayers:   4,
		GamePort:     7777,
		ReliablePort: 8888,
		SkipUpdate:   true,
		SteamBeta:    false,
		PUID:         1000,
		PGID:         1000,
		Image:        "ghcr.io/robtme/satisfactory-server",
	})
	for _, want := range []string{
		"SERVER_NAME=Factory",
		"MAXPLAYERS=4",
		"SERVERGAMEPORT=7777",
		"SERVERMESSAGINGPORT=8888",
		"SKIPUPDATE=true",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in:\n%s", want, got)
		}
	}
}

func TestConfigHashStable(t *testing.T) {
	s := ServerSettings{ServerName: "a", MaxPlayers: 4, GamePort: 7777, ReliablePort: 8888}
	h1 := ConfigHash(s)
	h2 := ConfigHash(s)
	if h1 != h2 {
		t.Fatal("hash not stable")
	}
	s2 := s
	s2.MaxPlayers = 8
	if h1 == ConfigHash(s2) {
		t.Fatal("hash should change when MaxPlayers changes")
	}
}