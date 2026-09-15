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
	"archive/tar"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

// fakeExec records what would have been run in the pod and keeps the stdin,
// which is the only way to check that the tar stream carries the tree.
type fakeExec struct {
	received []byte
	commands [][]string
}

func (f *fakeExec) run(_ string, command []string, stdin io.Reader) error {
	f.commands = append(f.commands, command)
	if stdin != nil {
		data, err := io.ReadAll(stdin)
		if err != nil {
			return err
		}
		f.received = data
	}
	return nil
}

func runningPodsImmediately(client *fake.Clientset) {
	client.PrependReactor("create", "pods", func(action k8stesting.Action) (bool, runtime.Object, error) {
		pod := action.(k8stesting.CreateAction).GetObject().(*corev1.Pod)
		pod.Status.Phase = corev1.PodRunning
		return false, nil, nil
	})
}

func writeProject(t *testing.T, cfg *testProject) string {
	t.Helper()
	root := t.TempDir()
	source := filepath.Join(root, ".horusec", cfg.analysisID)
	require.NoError(t, os.MkdirAll(filepath.Join(source, "pkg"), 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(source, "main.py"), []byte("import os\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(source, "pkg", "util.py"), []byte("x = 1\n"), 0o600))
	return root
}

type testProject struct{ analysisID string }

// Without a claim the pod must be free to land anywhere. Pinning it was what
// made analysers queue behind the caller on a single node until the analysis
// timed out.
func TestWithoutClaimThePodIsNotPinned(t *testing.T) {
	analysisID := uuid.New()
	cfg := newTestConfig()
	cfg.K8sWorkspaceClaim = ""
	cfg.ProjectPath = writeProject(t, &testProject{analysisID: analysisID.String()})

	client := fake.NewSimpleClientset()
	runningPodsImmediately(client)
	api := New(client, nil, cfg, analysisID)
	api.exec = (&fakeExec{}).run

	pod := api.buildPod(newTestData())

	assert.Nil(t, pod.Spec.NodeSelector, "pod was pinned to a node without a claim to anchor it")
	require.NotNil(t, pod.Spec.Volumes[0].EmptyDir, "workspace should be an emptyDir without a claim")
	assert.Empty(t, pod.Spec.Containers[0].VolumeMounts[0].SubPath)

	// The tool only starts once the code has arrived.
	assert.Contains(t, pod.Spec.Containers[0].Command[2], readyFlag)
	assert.Contains(t, pod.Spec.Containers[0].Command[2], "cd /src &&")
}

func TestStreamProjectIntoPod(t *testing.T) {
	analysisID := uuid.New()
	cfg := newTestConfig()
	cfg.K8sWorkspaceClaim = ""
	cfg.ProjectPath = writeProject(t, &testProject{analysisID: analysisID.String()})

	client := fake.NewSimpleClientset(&corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "horusec-python-abc", Namespace: "prod"},
		Status:     corev1.PodStatus{Phase: corev1.PodRunning},
	})
	api := New(client, nil, cfg, analysisID)
	exec := &fakeExec{}
	api.exec = exec.run

	require.NoError(t, api.streamProjectIntoPod("horusec-python-abc", api.projectSource()))

	// Unpack the archive, then release the tool. The order is what keeps the
	// tool from reading a half-unpacked tree.
	require.Len(t, exec.commands, 2)
	assert.Equal(t, []string{"tar", "-xf", "-", "-C", "/src"}, exec.commands[0])
	assert.Equal(t, []string{"touch", readyFlag}, exec.commands[1])

	names := tarEntries(t, exec.received)
	assert.Contains(t, names, "main.py")
	assert.Contains(t, names, "pkg/util.py")
	assert.Contains(t, names, "pkg")
}

// A limit alone becomes the request in Kubernetes, and a 1-CPU request per
// analyser is what left every pod Pending with "Insufficient cpu".
func TestResourcesKeepRequestsBelowLimits(t *testing.T) {
	cfg := newTestConfig()
	cfg.K8sPodCPULimit = "1"
	cfg.K8sPodMemoryLimit = "1Gi"
	cfg.K8sPodCPURequest = "100m"
	cfg.K8sPodMemoryRequest = "128Mi"
	api := New(fake.NewSimpleClientset(), nil, cfg, uuid.New())

	res := api.resources()

	assert.Equal(t, "1", res.Limits.Cpu().String())
	assert.Equal(t, "1Gi", res.Limits.Memory().String())
	assert.Equal(t, "100m", res.Requests.Cpu().String())
	assert.Equal(t, "128Mi", res.Requests.Memory().String())
}

func tarEntries(t *testing.T, archive []byte) []string {
	t.Helper()
	reader := tar.NewReader(bytes.NewReader(archive))
	names := make([]string, 0)
	for {
		header, err := reader.Next()
		if err == io.EOF {
			return names
		}
		require.NoError(t, err)
		names = append(names, header.Name)
	}
}
