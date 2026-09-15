// Copyright 2026 AUDAZ TECNOLOGIA
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package kubernetes runs each analyser as a short-lived Pod instead of a
// container created through a Docker daemon.
//
// The motivation is operational, not aesthetic. Running the CLI inside
// Kubernetes previously meant shipping a Docker-in-Docker sidecar, which is
// privileged and loads kernel modules on the host it lands on. In production
// that arrangement preceded two nodes losing their kubelet while the daemon sat
// idle and no analyser had run yet. Here there is no daemon: the kubelet
// creates the pods through the ordinary, supported path, and a misbehaving tool
// is bounded by that pod's own limits rather than by the node's patience.
//
// Everything above this layer is unchanged. Formatters keep building the same
// AnalysisData, the same images run the same shell CMD against the same /src,
// and the same parsers read the output.
package kubernetes

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/heron-brito/horusec-devkit/pkg/utils/logger"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/ZupIT/horusec/config"
	dockerentity "github.com/ZupIT/horusec/internal/entities/docker"
)

// ErrImageCmdRequired occurs when an image or a command is empty.
var ErrImageCmdRequired = errors.New("image or cmd is empty")

// ErrEmptyProjectPath occurs when there is no project to analyse. Failing here
// is deliberate: a pod that starts with an empty /src analyses nothing and
// reports clean, which is the worst possible outcome for a security scanner.
var ErrEmptyProjectPath = errors.New("kubernetes execution backend has no project path to copy")

const (
	// mountPath mirrors the Docker backend's pathDestinyInContainer. Several
	// CMDs and parsers hardcode /src — dependency-check scans it by absolute
	// path, and Service.RemoveSrcFolderFromPath strips it from reported paths.
	mountPath = "/src"

	// containerName is referenced by the exec calls that copy the project in.
	containerName = "analyser"

	// labelAnalysis marks every pod this analysis creates, so a timeout can
	// clean up whatever is still running without knowing their names.
	labelAnalysis  = "horusec.io/analysis-id"
	labelManagedBy = "app.kubernetes.io/managed-by"

	pollInterval = 2 * time.Second
)

// API is the Kubernetes implementation of the docker.Docker interface.
type API struct {
	ctx        context.Context
	client     kubernetes.Interface
	restConfig *rest.Config
	config     *config.Config
	analysisID uuid.UUID
	// exec is replaceable so the transfer path can be exercised without a
	// cluster. Nil means the real SPDY implementation.
	exec func(pod string, command []string, stdin io.Reader) error
}

func New(client kubernetes.Interface, restConfig *rest.Config, cfg *config.Config, analysisID uuid.UUID) *API {
	api := &API{
		ctx:        context.Background(),
		client:     client,
		restConfig: restConfig,
		config:     cfg,
		analysisID: analysisID,
	}
	return api
}

// PullImage is a no-op. The kubelet pulls the image when it starts the pod, and
// it does so with the node's credentials and its own cache. Reporting success
// here keeps the runner's existing pull-then-run sequence intact.
func (a *API) PullImage(_ string) error {
	return nil
}

// CreateLanguageAnalysisContainer runs one tool and returns everything it wrote.
func (a *API) CreateLanguageAnalysisContainer(data *dockerentity.AnalysisData) (string, error) {
	if data.IsInvalid() {
		return "", ErrImageCmdRequired
	}
	source := a.projectSource()
	if source == "" {
		return "", ErrEmptyProjectPath
	}

	pod, err := a.client.CoreV1().Pods(a.namespace()).Create(
		a.ctx, a.buildPod(data), metav1.CreateOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to create analysis pod: %w", err)
	}
	defer a.deletePod(pod.Name)

	// In claim mode the code is already on the volume the pod mounts. Without a
	// claim the pod starts empty and waits, and the code is streamed in — which
	// is what lets the scheduler place it on any node instead of the single one
	// a ReadWriteOnce volume is attached to.
	if !a.usesClaim() {
		if err := a.streamProjectIntoPod(pod.Name, source); err != nil {
			return "", err
		}
	}

	if err := a.waitForTermination(pod.Name); err != nil {
		return "", err
	}

	return a.podLogs(pod.Name)
}

func (a *API) usesClaim() bool {
	return a.config.K8sWorkspaceClaim != ""
}

// projectSource is the sanitized copy language detection already made.
func (a *API) projectSource() string {
	if a.config.ProjectPath == "" {
		return ""
	}
	return filepath.Join(a.config.ProjectPath, ".horusec", a.analysisID.String())
}

// DeleteContainersFromAPI removes whatever this analysis still has running. The
// runner calls it on timeout, when the pods are precisely what needs to go.
func (a *API) DeleteContainersFromAPI() {
	err := a.client.CoreV1().Pods(a.namespace()).DeleteCollection(
		a.ctx,
		metav1.DeleteOptions{},
		metav1.ListOptions{LabelSelector: fmt.Sprintf("%s=%s", labelAnalysis, a.analysisID.String())},
	)
	if err != nil && !k8serrors.IsNotFound(err) {
		logger.LogError("failed to delete analysis pods", err)
	}
}

func (a *API) buildPod(data *dockerentity.AnalysisData) *corev1.Pod {
	// `cd /src &&` and the ANALYSISID substitution are what the Docker backend
	// does before handing the string to /bin/sh; the CMDs are written expecting
	// both.
	cmd := strings.ReplaceAll(data.CMD, "ANALYSISID", a.analysisID.String())

	// A pod name has to be a DNS label, and the language is the only part of
	// this that a human reading `kubectl get pods` will care about.
	name := fmt.Sprintf("horusec-%s-%s",
		strings.ToLower(strings.ReplaceAll(data.Language.ToString(), " ", "")),
		uuid.New().String()[:8])

	falseVal := false
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: a.namespace(),
			Labels: map[string]string{
				labelAnalysis:  a.analysisID.String(),
				labelManagedBy: "horusec",
			},
		},
		Spec: corev1.PodSpec{
			RestartPolicy: corev1.RestartPolicyNever,
			// The analyser has no business talking to the API server, and the
			// token would be the most valuable thing in the pod.
			AutomountServiceAccountToken: &falseVal,
			// The workspace volume is ReadWriteOnce, so every pod that mounts
			// it has to land on the node that already has it attached.
			NodeSelector: a.nodeSelector(),
			Containers:   []corev1.Container{a.buildContainer(data, cmd)},
			Volumes:      []corev1.Volume{a.workspaceVolume()},
		},
	}
}

// workspaceVolume is a claim when one is configured and an empty directory
// otherwise. The empty directory is what the project gets streamed into.
//
// Not read-only either way: several CMDs write next to the code they scan.
// Bandit redirects its report into the working directory, which is /src, and a
// read-only mount would turn that into an empty result rather than an error.
func (a *API) workspaceVolume() corev1.Volume {
	if !a.usesClaim() {
		return corev1.Volume{
			Name:         "workspace",
			VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}},
		}
	}
	return corev1.Volume{
		Name: "workspace",
		VolumeSource: corev1.VolumeSource{
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
				ClaimName: a.config.K8sWorkspaceClaim,
			},
		},
	}
}

func (a *API) buildContainer(data *dockerentity.AnalysisData, cmd string) corev1.Container {
	mount := corev1.VolumeMount{Name: "workspace", MountPath: mountPath}
	if a.usesClaim() {
		mount.SubPath = a.workspaceSubPath()
	}

	return corev1.Container{
		Name:         containerName,
		Image:        data.GetCustomOrDefaultImage(),
		Command:      []string{"/bin/sh", "-c", a.shellScript(cmd)},
		Env:          a.containerEnv(),
		VolumeMounts: []corev1.VolumeMount{mount},
		Resources:    a.resources(),
	}
}

// shellScript is the tool invocation, preceded by a wait when the code still
// has to arrive. The wait is bounded so a transfer that never happens fails the
// pod instead of holding a slot until the whole analysis times out.
func (a *API) shellScript(cmd string) string {
	run := fmt.Sprintf("cd %s && %s", mountPath, cmd)
	if a.usesClaim() {
		return run
	}
	return fmt.Sprintf(
		"for i in $(seq 1 %d); do [ -f %s ] && break; sleep 1; done; "+
			"[ -f %s ] || { echo 'project was never copied into the analysis pod' >&2; exit 1; }; %s",
		a.config.TimeoutInSecondsAnalysis, readyFlag, readyFlag, run)
}

// workspaceSubPath locates, inside the shared volume, the sanitized copy that
// language detection already made at <ProjectPath>/.horusec/<analysisID>.
//
// The volume is mounted at K8sWorkspaceRoot in the process that runs the CLI,
// so the path relative to that root is the same path relative to the volume.
func (a *API) workspaceSubPath() string {
	full := filepath.Join(a.config.ProjectPath, ".horusec", a.analysisID.String())
	rel, err := filepath.Rel(a.config.K8sWorkspaceRoot, full)
	if err != nil || strings.HasPrefix(rel, "..") {
		// Outside the volume: mounting the root would silently analyse the
		// wrong tree, so mount nothing and let the tool report an empty dir.
		logger.LogError("project path is outside the workspace volume",
			fmt.Errorf("project %q is not under %q", full, a.config.K8sWorkspaceRoot))
		return ""
	}
	return rel
}

// containerEnv forwards the same three variables the Docker backend forwards.
// Nancy refuses to run without OSS Index credentials and npm audit uses the
// GitHub token; nothing else the worker holds belongs in an analyser.
func (a *API) containerEnv() []corev1.EnvVar {
	names := []string{"GITHUB_TOKEN", "OSSINDEX_USERNAME", "OSSINDEX_TOKEN"}
	env := make([]corev1.EnvVar, 0, len(names))
	for _, name := range names {
		if value := os.Getenv(name); value != "" {
			env = append(env, corev1.EnvVar{Name: name, Value: value})
		}
	}
	return env
}

// resources keeps requests and limits apart, and that distinction is the whole
// point.
//
// Kubernetes copies the limit into the request when only a limit is given. A
// 1-CPU limit then means every analyser *reserves* a full core, so a node with
// 1830m allocatable fits one of them and the rest sit Pending until the
// analysis times out. That is not a hypothesis: it is what a 1-CPU limit did in
// production, with the scheduler reporting "Insufficient cpu".
//
// The request is what the scheduler packs against and should describe the idle
// footprint; the limit is the ceiling that contains a tool going wrong.
func (a *API) resources() corev1.ResourceRequirements {
	quantities := func(cpu, memory string) corev1.ResourceList {
		list := corev1.ResourceList{}
		if cpu != "" {
			if q, err := resource.ParseQuantity(cpu); err == nil {
				list[corev1.ResourceCPU] = q
			}
		}
		if memory != "" {
			if q, err := resource.ParseQuantity(memory); err == nil {
				list[corev1.ResourceMemory] = q
			}
		}
		return list
	}

	req := corev1.ResourceRequirements{}
	if limits := quantities(a.config.K8sPodCPULimit, a.config.K8sPodMemoryLimit); len(limits) > 0 {
		req.Limits = limits
	}
	if requests := quantities(a.config.K8sPodCPURequest, a.config.K8sPodMemoryRequest); len(requests) > 0 {
		req.Requests = requests
	}
	return req
}

// waitForTermination blocks until the pod stops, either way.
//
// A non-zero exit is not an error here. Half the tools exit non-zero when they
// find something — gitleaks and checkov among them — and the CMDs are written
// to print the report regardless. Treating that as a failure would discard
// exactly the runs that found vulnerabilities.
func (a *API) waitForTermination(name string) error {
	deadline := time.Now().Add(time.Duration(a.config.TimeoutInSecondsAnalysis) * time.Second)

	for {
		pod, err := a.client.CoreV1().Pods(a.namespace()).Get(a.ctx, name, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("failed to read analysis pod %s: %w", name, err)
		}

		switch pod.Status.Phase {
		case corev1.PodSucceeded, corev1.PodFailed:
			return nil
		case corev1.PodPending, corev1.PodRunning, corev1.PodUnknown:
		}

		if time.Now().After(deadline) {
			return fmt.Errorf("analysis pod %s did not finish within %d seconds",
				name, a.config.TimeoutInSecondsAnalysis)
		}
		time.Sleep(pollInterval)
	}
}

// podLogs returns stdout and stderr as a single string.
//
// This is not a convenience: the Docker backend created its containers with
// Tty: true, which merges the two streams, and several parsers depend on that.
// phpcs says so in its config, and checkov compares against "[]\r\n". Reading
// only stdout here would break them in ways that look like empty results.
func (a *API) podLogs(name string) (string, error) {
	stream, err := a.client.CoreV1().Pods(a.namespace()).
		GetLogs(name, &corev1.PodLogOptions{}).Stream(a.ctx)
	if err != nil {
		return "", fmt.Errorf("failed to read logs of analysis pod %s: %w", name, err)
	}
	defer func() { _ = stream.Close() }()

	output, err := io.ReadAll(stream)
	if err != nil {
		return "", fmt.Errorf("failed to read logs of analysis pod %s: %w", name, err)
	}
	return string(output), nil
}

func (a *API) deletePod(name string) {
	err := a.client.CoreV1().Pods(a.namespace()).Delete(a.ctx, name, metav1.DeleteOptions{})
	if err != nil && !k8serrors.IsNotFound(err) {
		logger.LogError(fmt.Sprintf("failed to delete analysis pod %s", name), err)
	}
}

// nodeSelector pins the pod to the node that has the workspace volume
// attached. `kubernetes.io/hostname` rather than PodSpec.NodeName on purpose:
// NodeName bypasses the scheduler entirely, so a node without room would take
// the pod anyway and fail it late, while a selector lets the scheduler apply
// the usual resource and taint checks.
func (a *API) nodeSelector() map[string]string {
	// Without a claim there is nothing anchoring the pod to a node, and
	// pinning it would recreate the bottleneck this mode exists to avoid.
	if !a.usesClaim() || a.config.K8sNodeName == "" {
		return nil
	}
	return map[string]string{"kubernetes.io/hostname": a.config.K8sNodeName}
}

func (a *API) namespace() string {
	if a.config.K8sNamespace == "" {
		return "default"
	}
	return a.config.K8sNamespace
}
