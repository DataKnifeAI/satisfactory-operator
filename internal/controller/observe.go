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

package controller

import (
	"context"
	"io"

	satisfactoryv1alpha1 "github.com/DataKnifeAI/satisfactory-operator/api/v1alpha1"
	"github.com/DataKnifeAI/satisfactory-operator/internal/config"
	"github.com/DataKnifeAI/satisfactory-operator/internal/provisioner"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type observedRuntime struct {
	WorldSaveGuid string
	BuildID       string
}

func (r *SatisfactoryServerReconciler) observeRuntime(
	ctx context.Context,
	server *satisfactoryv1alpha1.SatisfactoryServer,
) observedRuntime {
	var out observedRuntime
	if r.Kube == nil {
		return out
	}
	podList := &corev1.PodList{}
	if err := r.List(ctx, podList,
		client.InNamespace(server.Namespace),
		client.MatchingLabels(map[string]string{
			provisioner.LabelAppInstance:  server.Name,
			provisioner.LabelAppComponent: provisioner.ComponentGame,
		}),
	); err != nil || len(podList.Items) == 0 {
		return out
	}
	pod := podList.Items[0]
	if pod.Status.Phase != corev1.PodRunning {
		return out
	}
	tail := int64(300)
	req := r.Kube.CoreV1().Pods(pod.Namespace).GetLogs(pod.Name, &corev1.PodLogOptions{
		Container: "satisfactory",
		TailLines: &tail,
	})
	stream, err := req.Stream(ctx)
	if err != nil {
		return out
	}
	defer stream.Close() //nolint:errcheck
	buf, err := io.ReadAll(io.LimitReader(stream, 512*1024))
	if err != nil {
		return out
	}
	text := string(buf)
	out.WorldSaveGuid = config.ParseWorldSaveGuid(text)
	out.BuildID = config.ParseBuildID(text)
	return out
}
