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
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"

	satisfactoryv1alpha1 "github.com/DataKnifeAI/satisfactory-operator/api/v1alpha1"
)

// ServerSettings is non-secret config mirrored into a ConfigMap for observability
// and used to hash pod roll annotations. Passwords are never stored here.
type ServerSettings struct {
	ServerName    string
	MaxPlayers    int32
	GamePort      int32
	ReliablePort  int32
	SkipUpdate    bool
	SteamBeta     bool
	PUID          int64
	PGID          int64
	Image         string
}

// SettingsFromSpec builds settings from the CR.
func SettingsFromSpec(server *satisfactoryv1alpha1.SatisfactoryServer) ServerSettings {
	return ServerSettings{
		ServerName:   ServerName(server.Spec, server.Name),
		MaxPlayers:   MaxPlayers(server.Spec),
		GamePort:     GamePort(server.Spec),
		ReliablePort: ReliablePort(server.Spec),
		SkipUpdate:   SkipUpdate(server.Spec),
		SteamBeta:    server.Spec.Update.SteamBeta,
		PUID:         PUID(server.Spec),
		PGID:         PGID(server.Spec),
		Image:        ImageRef(server.Spec),
	}
}

// RenderSettingsEnv renders a simple KEY=VALUE file for the ConfigMap.
func RenderSettingsEnv(s ServerSettings) string {
	var b strings.Builder
	fmt.Fprintf(&b, "SERVER_NAME=%s\n", s.ServerName)
	fmt.Fprintf(&b, "MAXPLAYERS=%d\n", s.MaxPlayers)
	fmt.Fprintf(&b, "SERVERGAMEPORT=%d\n", s.GamePort)
	fmt.Fprintf(&b, "SERVERMESSAGINGPORT=%d\n", s.ReliablePort)
	fmt.Fprintf(&b, "SKIPUPDATE=%s\n", strconv.FormatBool(s.SkipUpdate))
	fmt.Fprintf(&b, "STEAMBETA=%s\n", strconv.FormatBool(s.SteamBeta))
	fmt.Fprintf(&b, "PUID=%d\n", s.PUID)
	fmt.Fprintf(&b, "PGID=%d\n", s.PGID)
	fmt.Fprintf(&b, "IMAGE=%s\n", s.Image)
	return b.String()
}

// ConfigHash returns a short hash of non-secret config used to annotate pods.
func ConfigHash(s ServerSettings) string {
	sum := sha256.Sum256([]byte(RenderSettingsEnv(s)))
	return hex.EncodeToString(sum[:8])
}
