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
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/remotecommand"
)

// The pod waits for this file before running the tool. It lives in the
// container's own /tmp, never in /src: a stray file inside the analysed tree
// would show up in the tool's own findings.
const readyFlag = "/tmp/.horusec-ready"

// streamProjectIntoPod copies the project into a pod that is already running.
//
// This is what frees the scheduler. The alternative — a ReadWriteOnce claim
// shared with the caller — works, but ReadWriteOnce means one node, so every
// analyser has to be pinned there. On a cluster where that node is also running
// the caller, the analysers queue behind it instead of spreading out.
//
// The cost is that the pod cannot simply run the tool on start: it has to wait
// for the code to arrive. Hence the ready flag.
func (a *API) streamProjectIntoPod(name, source string) error {
	if err := a.waitForRunning(name); err != nil {
		return err
	}

	archive, err := tarDirectory(source)
	if err != nil {
		return fmt.Errorf("failed to pack %s: %w", source, err)
	}

	if err := a.execInPod(name, []string{"tar", "-xf", "-", "-C", mountPath}, archive); err != nil {
		return fmt.Errorf("failed to copy the project into analysis pod %s: %w", name, err)
	}

	// Only now may the tool start. Touching the flag last is what makes the
	// transfer atomic from the tool's point of view — it never sees a half
	// unpacked tree and reports it as the whole repository.
	if err := a.execInPod(name, []string{"touch", readyFlag}, nil); err != nil {
		return fmt.Errorf("failed to release analysis pod %s: %w", name, err)
	}
	return nil
}

// waitForRunning blocks until the container is up and can be exec'd into.
func (a *API) waitForRunning(name string) error {
	deadline := time.Now().Add(time.Duration(a.config.TimeoutInSecondsAnalysis) * time.Second)

	for {
		pod, err := a.client.CoreV1().Pods(a.namespace()).Get(a.ctx, name, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("failed to read analysis pod %s: %w", name, err)
		}

		switch pod.Status.Phase {
		case corev1.PodRunning:
			return nil
		case corev1.PodSucceeded, corev1.PodFailed:
			// The container is gone before the code ever arrived, which means
			// it failed on its own — a missing image, most likely.
			return fmt.Errorf("analysis pod %s terminated before the project was copied", name)
		case corev1.PodPending, corev1.PodUnknown:
		}

		if time.Now().After(deadline) {
			return fmt.Errorf("analysis pod %s was not scheduled within %d seconds: %s",
				name, a.config.TimeoutInSecondsAnalysis, schedulingHint(pod))
		}
		time.Sleep(pollInterval)
	}
}

// schedulingHint turns a Pending pod into something actionable. Without it a
// timeout says only that nothing happened, and the usual cause — the pod does
// not fit anywhere — is written down in the conditions the scheduler leaves.
func schedulingHint(pod *corev1.Pod) string {
	for _, cond := range pod.Status.Conditions {
		if cond.Type == corev1.PodScheduled && cond.Status == corev1.ConditionFalse {
			return strings.TrimSpace(cond.Message)
		}
	}
	return string(pod.Status.Phase)
}

// execInPod runs a command inside the analyser container.
//
// It is a field on API rather than a plain method so tests can replace it: the
// real implementation needs a SPDY connection to a live API server, which the
// fake clientset has none of, and that would leave the only part of this file
// with moving parts uncovered.
func (a *API) execInPod(name string, command []string, stdin io.Reader) error {
	if a.exec != nil {
		return a.exec(name, command, stdin)
	}
	return a.execViaSPDY(name, command, stdin)
}

func (a *API) execViaSPDY(name string, command []string, stdin io.Reader) error {
	req := a.client.CoreV1().RESTClient().Post().
		Resource("pods").Name(name).Namespace(a.namespace()).SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Container: containerName,
			Command:   command,
			Stdin:     stdin != nil,
			Stdout:    true,
			Stderr:    true,
		}, scheme.ParameterCodec)

	executor, err := remotecommand.NewSPDYExecutor(a.restConfig, "POST", req.URL())
	if err != nil {
		return err
	}

	var stderr bytes.Buffer
	err = executor.StreamWithContext(a.ctx, remotecommand.StreamOptions{
		Stdin:  stdin,
		Stdout: io.Discard,
		Stderr: &stderr,
	})
	if err != nil {
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return nil
}

// tarDirectory packs a directory into an in-memory tar stream.
//
// In memory because the thing being packed is a source tree that language
// detection already filtered, and because writing a temporary file would need
// somewhere to put it — the one resource this backend exists to stop competing
// for.
func tarDirectory(root string) (io.Reader, error) {
	var buf bytes.Buffer
	writer := tar.NewWriter(&buf)

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		// Symlinks are skipped rather than followed: a link pointing outside
		// the tree would copy something the analysis was never meant to see.
		if !info.Mode().IsRegular() && !info.IsDir() {
			return nil
		}

		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}

		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		header.Name = filepath.ToSlash(rel)
		if err := writer.WriteHeader(header); err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer func() { _ = file.Close() }()

		_, err = io.Copy(writer, file)
		return err
	})
	if err != nil {
		return nil, err
	}

	if err := writer.Close(); err != nil {
		return nil, err
	}
	return &buf, nil
}
