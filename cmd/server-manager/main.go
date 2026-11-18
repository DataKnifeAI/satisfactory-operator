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

package main

import (
	"flag"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/DataKnifeAI/satisfactory-operator/internal/servermanager"
)

func envOr(keys []string, fallback string) string {
	for _, key := range keys {
		if v := os.Getenv(key); v != "" {
			return v
		}
	}
	return fallback
}

func main() {
	listen := flag.String("listen", envOr([]string{"SERVER_MANAGER_LISTEN"}, ":8088"), "listen address")
	data := flag.String("data", envOr([]string{"SERVER_MANAGER_DATA"}, "/data"), "game data PVC mount")
	backups := flag.String("backups", envOr([]string{"SERVER_MANAGER_BACKUPS"}, "/backups"), "backups PVC mount (optional)")
	user := flag.String("user", envOr([]string{"SERVER_MANAGER_USER"}, servermanager.DefaultUser), "basic auth username")
	flag.Parse()

	password := envOr([]string{"SERVER_MANAGER_PASSWORD"}, "")
	if password == "" {
		log.Fatal("SERVER_MANAGER_PASSWORD is required")
	}

	namespace := envOr([]string{"SERVER_MANAGER_NAMESPACE"}, "")
	crName := envOr([]string{"SERVER_MANAGER_CR"}, "")
	deploy := envOr([]string{"SERVER_MANAGER_DEPLOYMENT"}, crName)
	secret := envOr([]string{"SERVER_MANAGER_SECRET"}, "")

	var runtime *servermanager.Runtime
	if namespace != "" && crName != "" {
		rt, err := servermanager.NewRuntime(namespace, crName, deploy, secret)
		if err != nil {
			log.Printf("kubernetes client unavailable: %v", err)
		} else {
			runtime = rt
		}
	}

	srv, err := servermanager.New(servermanager.Config{
		User:     *user,
		Password: password,
		DataRoot: *data,
		Backups:  *backups,
		Runtime:  runtime,
	})
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("server manager listening on %s data=%s backups=%s", *listen, *data, *backups)
	httpSrv := &http.Server{
		Addr:              *listen,
		Handler:           srv,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
	if err := httpSrv.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}
