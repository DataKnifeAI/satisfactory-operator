/*
Copyright 2026 DataKnifeAI.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

    10|Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package servermanager

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	satisfactoryv1alpha1 "github.com/DataKnifeAI/satisfactory-operator/api/v1alpha1"
)

const DefaultUser = "admin"

// Config wires the HTTP admin UI.
type Config struct {
	User     string
	Password string
	DataRoot string // game PVC mount (e.g. /data)
	Backups  string // backup PVC mount (optional)
	Runtime  *Runtime
}

// Server is the authenticated admin UI/API.
type Server struct {
	cfg Config
	mux *http.ServeMux
}

// New builds the Server. Password must be non-empty.
func New(cfg Config) (*Server, error) {
	if cfg.Password == "" {
		return nil, errors.New("server manager password is required")
	}
	if cfg.User == "" {
		cfg.User = DefaultUser
	}
	if cfg.DataRoot == "" {
		cfg.DataRoot = "/data"
	}
	s := &Server{cfg: cfg, mux: http.NewServeMux()}
	s.routes()
	return s, nil
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /{$}", s.withAuth(s.handleUI))
	s.mux.Handle("GET /assets/", http.StripPrefix("/assets/", s.withAuthHandler(assetHandler())))
	s.mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	s.mux.HandleFunc("GET /logout", s.handleLogout)
	s.mux.HandleFunc("GET /api/status", s.withAuth(s.handleStatus))
	s.mux.HandleFunc("POST /api/restart", s.withAuth(s.handleRestart))
	s.mux.HandleFunc("POST /api/settings", s.withAuth(s.handleSettings))
	s.mux.HandleFunc("POST /api/password", s.withAuth(s.handlePassword))
	s.mux.HandleFunc("GET /api/saves/download", s.withAuth(s.handleSavesDownload))
	s.mux.HandleFunc("GET /api/backups", s.withAuth(s.handleBackupsList))
	s.mux.HandleFunc("GET /api/backups/download", s.withAuth(s.handleBackupDownload))
	s.mux.HandleFunc("POST /api/backups/run", s.withAuth(s.handleBackupRun))
}

func (s *Server) withAuth(next http.HandlerFunc) http.HandlerFunc {
	return s.withAuthHandler(next).ServeHTTP
}

func (s *Server) withAuthHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if !ok ||
			subtle.ConstantTimeCompare([]byte(user), []byte(s.cfg.User)) != 1 ||
			subtle.ConstantTimeCompare([]byte(pass), []byte(s.cfg.Password)) != 1 {
			w.Header().Set("WWW-Authenticate", `Basic realm="Satisfactory Server Manager"`)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) handleLogout(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("WWW-Authenticate", `Basic realm="Satisfactory Server Manager"`)
	http.Error(w, "logged out", http.StatusUnauthorized)
}

func (s *Server) handleUI(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(uiHTML))
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	if s.cfg.Runtime == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "kubernetes client unavailable"})
		return
	}
	server, err := s.cfg.Runtime.GetServer(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	skipUpdate := true
	if server.Spec.Update.SkipUpdate != nil {
		skipUpdate = *server.Spec.Update.SkipUpdate
	}
	displayName := server.Spec.ServerName
	if displayName == "" {
		displayName = server.Status.ServerName
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"name":              server.Name,
		"namespace":         server.Namespace,
		"phase":             server.Status.Phase,
		"ready":             server.Status.Ready,
		"message":           server.Status.Message,
		"connectionAddress": server.Status.ConnectionAddress,
		"connectionPort":    server.Status.ConnectionPort,
		"reliablePort":      server.Status.ReliablePort,
		"imageVersion":      server.Status.ImageVersion,
		"serverManagerPort": server.Status.ServerManagerPort,
		"serverName":        displayName,
		"maxPlayers":        server.Spec.MaxPlayers,
		"skipUpdate":        skipUpdate,
		"steamBeta":         server.Spec.Update.SteamBeta,
		"autoUpdateImage":   server.Spec.Update.AutoUpdateImage,
		"backupEnabled":     server.Spec.Backup.Enabled,
	})
}

func (s *Server) handleRestart(w http.ResponseWriter, r *http.Request) {
	if s.cfg.Runtime == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "kubernetes client unavailable"})
		return
	}
	if err := s.cfg.Runtime.Restart(r.Context()); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "message": "Deployment restart requested (Recreate)"})
}

type settingsBody struct {
	ServerName      *string `json:"serverName"`
	MaxPlayers      *int32  `json:"maxPlayers"`
	SkipUpdate      *bool   `json:"skipUpdate"`
	SteamBeta       *bool   `json:"steamBeta"`
	AutoUpdateImage *bool   `json:"autoUpdateImage"`
}

func (s *Server) handleSettings(w http.ResponseWriter, r *http.Request) {
	if s.cfg.Runtime == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "kubernetes client unavailable"})
		return
	}
	var body settingsBody
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid JSON"})
		return
	}
	server, err := s.cfg.Runtime.GetServer(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	if body.ServerName != nil {
		server.Spec.ServerName = *body.ServerName
	}
	if body.MaxPlayers != nil {
		if *body.MaxPlayers < 1 || *body.MaxPlayers > 16 {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "maxPlayers must be 1–16"})
			return
		}
		server.Spec.MaxPlayers = *body.MaxPlayers
	}
	if body.SkipUpdate != nil {
		server.Spec.Update.SkipUpdate = body.SkipUpdate
	}
	if body.SteamBeta != nil {
		server.Spec.Update.SteamBeta = *body.SteamBeta
	}
	if body.AutoUpdateImage != nil {
		server.Spec.Update.AutoUpdateImage = *body.AutoUpdateImage
	}
	if err := s.cfg.Runtime.UpdateServer(r.Context(), server); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "message": "settings saved — restart to apply env"})
}

type passwordBody struct {
	Password string `json:"password"`
	Generate bool   `json:"generate"`
}

func (s *Server) handlePassword(w http.ResponseWriter, r *http.Request) {
	if s.cfg.Runtime == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "kubernetes client unavailable"})
		return
	}
	var body passwordBody
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid JSON"})
		return
	}
	key := satisfactoryv1alpha1.SecretKeyAdminPassword
	pass := body.Password
	if body.Generate {
		b := make([]byte, 12)
		if _, err := rand.Read(b); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
			return
		}
		pass = hex.EncodeToString(b)
	}
	secret, err := s.cfg.Runtime.GetSecret(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	if secret.Data == nil {
		secret.Data = map[string][]byte{}
	}
	secret.Data[key] = []byte(pass)
	if err := s.cfg.Runtime.UpdateSecret(r.Context(), secret); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	out := map[string]any{"ok": true, "message": "password updated — restart to apply"}
	if body.Generate {
		out["password"] = pass
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleSavesDownload(w http.ResponseWriter, r *http.Request) {
	root := s.cfg.DataRoot
	if _, err := os.Stat(root); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "data volume not mounted"})
		return
	}
	name := fmt.Sprintf("satisfactory-saves-%s.tar.gz", time.Now().UTC().Format("20060102T150405Z"))
	w.Header().Set("Content-Type", "application/gzip")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, name))
	if err := writeTarGz(w, root); err != nil {
		// headers may already be sent
		return
	}
}

func (s *Server) handleBackupsList(w http.ResponseWriter, r *http.Request) {
	dir := s.cfg.Backups
	if dir == "" {
		writeJSON(w, http.StatusOK, map[string]any{"backups": []any{}, "note": "backup volume not mounted"})
		return
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"backups": []any{}, "note": err.Error()})
		return
	}
	type item struct {
		Name string `json:"name"`
		Size int64  `json:"size"`
	}
	var list []item
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".tar.gz") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		list = append(list, item{Name: e.Name(), Size: info.Size()})
	}
	writeJSON(w, http.StatusOK, map[string]any{"backups": list})
}

func (s *Server) handleBackupDownload(w http.ResponseWriter, r *http.Request) {
	name := filepath.Base(r.URL.Query().Get("name"))
	if name == "" || name != filepath.Base(name) || !strings.HasSuffix(name, ".tar.gz") {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid name"})
		return
	}
	path := filepath.Join(s.cfg.Backups, name)
	f, err := os.Open(path)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "not found"})
		return
	}
	defer f.Close()
	w.Header().Set("Content-Type", "application/gzip")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, name))
	_, _ = io.Copy(w, f)
}

func (s *Server) handleBackupRun(w http.ResponseWriter, r *http.Request) {
	if s.cfg.Runtime == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "kubernetes client unavailable"})
		return
	}
	name, err := s.cfg.Runtime.TriggerBackup(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "job": name})
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
