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

package provisioner

import (
	"fmt"
	"strconv"

	satisfactoryv1alpha1 "github.com/DataKnifeAI/satisfactory-operator/api/v1alpha1"
	"github.com/DataKnifeAI/satisfactory-operator/internal/config"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const ComponentBackup = "backup"

func backupLabels(server *satisfactoryv1alpha1.SatisfactoryServer) map[string]string {
	return map[string]string{
		LabelAppName:      "satisfactory-server",
		LabelAppInstance:  server.Name,
		LabelAppManagedBy: ManagedBy,
		LabelAppComponent: ComponentBackup,
	}
}

func backupPVCName(server *satisfactoryv1alpha1.SatisfactoryServer) string {
	return server.Name + "-backups"
}

func backupCronJobName(server *satisfactoryv1alpha1.SatisfactoryServer) string {
	return server.Name + "-backup"
}

// BackupPVCNameFor returns the backup PVC name.
func BackupPVCNameFor(server *satisfactoryv1alpha1.SatisfactoryServer) string {
	return backupPVCName(server)
}

// BackupCronJobNameFor returns the backup CronJob name.
func BackupCronJobNameFor(server *satisfactoryv1alpha1.SatisfactoryServer) string {
	return backupCronJobName(server)
}

// BuildBackupPVC creates a PVC that stores tar.gz archives of the data volume.
func BuildBackupPVC(server *satisfactoryv1alpha1.SatisfactoryServer) *corev1.PersistentVolumeClaim {
	size := config.BackupSize(server.Spec)
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      backupPVCName(server),
			Namespace: server.Namespace,
			Labels:    backupLabels(server),
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse(size),
				},
			},
		},
	}
	if sc := config.BackupStorageClass(server.Spec); sc != "" {
		pvc.Spec.StorageClassName = &sc
	}
	return pvc
}

// BuildBackupCronJob creates a CronJob that tars /data into /backups and prunes old archives.
func BuildBackupCronJob(server *satisfactoryv1alpha1.SatisfactoryServer) *batchv1.CronJob {
	retain := config.BackupRetain(server.Spec)
	script := fmt.Sprintf(`set -euo pipefail
STAMP=$(date -u +%%Y%%m%%dT%%H%%M%%SZ)
DEST=/backups/${STAMP}.tar.gz
echo "backing up /data -> ${DEST}"
tar -C /data -czf "${DEST}" .
echo "pruning to last %d archives"
ls -1t /backups/*.tar.gz 2>/dev/null | tail -n +%s | xargs -r rm -f
ls -lah /backups/
`, retain, strconv.FormatInt(int64(retain)+1, 10))

	labels := backupLabels(server)
	backoff := int32(1)
	ttl := int32(86400)
	successful := int32(3)
	failed := int32(1)
	concurrency := batchv1.ForbidConcurrent

	return &batchv1.CronJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      backupCronJobName(server),
			Namespace: server.Namespace,
			Labels:    labels,
		},
		Spec: batchv1.CronJobSpec{
			Schedule:                   config.BackupSchedule(server.Spec),
			ConcurrencyPolicy:          concurrency,
			SuccessfulJobsHistoryLimit: &successful,
			FailedJobsHistoryLimit:     &failed,
			JobTemplate: batchv1.JobTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: labels},
				Spec: batchv1.JobSpec{
					BackoffLimit:            &backoff,
					TTLSecondsAfterFinished: &ttl,
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{Labels: labels},
						Spec: corev1.PodSpec{
							RestartPolicy: corev1.RestartPolicyOnFailure,
							NodeSelector:  server.Spec.NodeSelector,
							Containers: []corev1.Container{
								{
									Name:            "backup",
									Image:           config.BackupImage(server.Spec),
									ImagePullPolicy: corev1.PullIfNotPresent,
									Command:         []string{"sh", "-c", script},
									VolumeMounts: []corev1.VolumeMount{
										{Name: "data", MountPath: "/data", ReadOnly: true},
										{Name: "backups", MountPath: "/backups"},
									},
								},
							},
							Volumes: []corev1.Volume{
								{
									Name: "data",
									VolumeSource: corev1.VolumeSource{
										PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
											ClaimName: pvcName(server),
											ReadOnly:  true,
										},
									},
								},
								{
									Name: "backups",
									VolumeSource: corev1.VolumeSource{
										PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
											ClaimName: backupPVCName(server),
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
}
