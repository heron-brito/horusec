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

package kubernetes

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/heron-brito/horusec-devkit/pkg/enums/languages"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"

	cliConfig "github.com/ZupIT/horusec/config"
	dockerEntities "github.com/ZupIT/horusec/internal/entities/docker"
)

func newTestConfig() *cliConfig.Config {
	cfg := cliConfig.New()
	cfg.ProjectPath = "/tmp/myproject_github"
	cfg.K8sWorkspaceRoot = "/tmp"
	cfg.K8sWorkspaceClaim = "kratos-workspace"
	cfg.K8sNamespace = "prod"
	cfg.K8sNodeName = "node-1"
	cfg.TimeoutInSecondsAnalysis = 60
	return cfg
}

func newTestData() *dockerEntities.AnalysisData {
	return &dockerEntities.AnalysisData{
		CMD:          "bandit -r . -o results-ANALYSISID.json",
		Language:     languages.Python,
		DefaultImage: "ghcr.io/heron-brito/horusec-python:v2.10.16",
	}
}

// succeedPodsImmediately makes every created pod report Succeeded, so the
// polling loop terminates. The fake clientset does not run a kubelet.
func succeedPodsImmediately(client *fake.Clientset) {
	client.PrependReactor("create", "pods", func(action k8stesting.Action) (bool, runtime.Object, error) {
		pod := action.(k8stesting.CreateAction).GetObject().(*corev1.Pod)
		pod.Status.Phase = corev1.PodSucceeded
		return false, nil, nil
	})
}

func TestCreateLanguageAnalysisContainer(t *testing.T) {
	t.Run("builds a pod that reproduces the docker contract", func(t *testing.T) {
		client := fake.NewSimpleClientset()
		succeedPodsImmediately(client)
		api := New(client, newTestConfig(), uuid.MustParse("11111111-1111-1111-1111-111111111111"))

		_, err := api.CreateLanguageAnalysisContainer(newTestData())
		require.NoError(t, err)

		var created *corev1.Pod
		for _, action := range client.Actions() {
			if create, ok := action.(k8stesting.CreateAction); ok && action.GetResource().Resource == "pods" {
				created = create.GetObject().(*corev1.Pod)
			}
		}
		require.NotNil(t, created, "no pod was created")

		container := created.Spec.Containers[0]
		assert.Equal(t, "ghcr.io/heron-brito/horusec-python:v2.10.16", container.Image)

		// The CMDs are written assuming the working directory is /src and that
		// ANALYSISID has already been substituted.
		assert.Equal(t, []string{
			"/bin/sh", "-c",
			"cd /src && bandit -r . -o results-11111111-1111-1111-1111-111111111111.json",
		}, container.Command)

		// The project lives at <workspace root>/<relative path>/.horusec/<id>,
		// and only that subtree may be mounted.
		assert.Equal(t, "/src", container.VolumeMounts[0].MountPath)
		assert.Equal(t,
			"myproject_github/.horusec/11111111-1111-1111-1111-111111111111",
			container.VolumeMounts[0].SubPath)

		// Writable on purpose: bandit redirects its report into the working
		// directory, which is the mount.
		assert.False(t, container.VolumeMounts[0].ReadOnly)

		assert.Equal(t, corev1.RestartPolicyNever, created.Spec.RestartPolicy)
		assert.Equal(t, "node-1", created.Spec.NodeSelector["kubernetes.io/hostname"])
		assert.False(t, *created.Spec.AutomountServiceAccountToken)
		assert.Equal(t, "prod", created.Namespace)
	})

	t.Run("deletes the pod once it has been read", func(t *testing.T) {
		client := fake.NewSimpleClientset()
		succeedPodsImmediately(client)
		api := New(client, newTestConfig(), uuid.New())

		_, err := api.CreateLanguageAnalysisContainer(newTestData())
		require.NoError(t, err)

		remaining, err := client.CoreV1().Pods("prod").List(context.Background(), metav1.ListOptions{})
		require.NoError(t, err)
		assert.Empty(t, remaining.Items, "analysis pod was left behind")
	})

	t.Run("refuses to run without a workspace claim", func(t *testing.T) {
		cfg := newTestConfig()
		cfg.K8sWorkspaceClaim = ""
		api := New(fake.NewSimpleClientset(), cfg, uuid.New())

		_, err := api.CreateLanguageAnalysisContainer(newTestData())
		assert.ErrorIs(t, err, ErrWorkspaceClaimRequired)
	})

	t.Run("refuses an analysis data without image or cmd", func(t *testing.T) {
		api := New(fake.NewSimpleClientset(), newTestConfig(), uuid.New())

		_, err := api.CreateLanguageAnalysisContainer(&dockerEntities.AnalysisData{})
		assert.ErrorIs(t, err, ErrImageCmdRequired)
	})
}

// A project outside the mounted volume must not fall back to mounting the
// volume root: that would hand the tool a different tree and report its
// findings as if they came from this repository.
func TestWorkspaceSubPathOutsideTheVolume(t *testing.T) {
	cfg := newTestConfig()
	cfg.ProjectPath = "/somewhere/else"
	api := New(fake.NewSimpleClientset(), cfg, uuid.New())

	assert.Empty(t, api.workspaceSubPath())
}

func TestPullImageIsANoOp(t *testing.T) {
	api := New(fake.NewSimpleClientset(), newTestConfig(), uuid.New())
	assert.NoError(t, api.PullImage("ghcr.io/heron-brito/horusec-python:v2.10.16"))
}

func TestDeleteContainersFromAPI(t *testing.T) {
	analysisID := uuid.New()
	client := fake.NewSimpleClientset()
	succeedPodsImmediately(client)
	api := New(client, newTestConfig(), analysisID)

	_, err := api.CreateLanguageAnalysisContainer(newTestData())
	require.NoError(t, err)

	// Does not panic and does not error when there is nothing left to remove.
	api.DeleteContainersFromAPI()
}
