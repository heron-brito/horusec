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

package client

import (
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// NewInClusterClient builds a Kubernetes client for the Horusec process.
//
// In-cluster first, because that is where this backend earns its keep: the
// service account token mounted in the pod is what lets the CLI create analyser
// pods next to itself. Outside a cluster it falls back to the usual kubeconfig
// resolution, which is what makes the backend testable from a developer machine
// against a real cluster.
//
// Returning the error rather than panicking is deliberate — the caller decides
// whether an unreachable cluster is fatal or a reason to use another backend.
// The rest config comes back alongside the clientset because copying files into
// a pod needs it: the exec subresource is a streaming connection, which the
// typed client does not build.
func NewInClusterClient() (kubernetes.Interface, *rest.Config, error) {
	cfg, err := rest.InClusterConfig()
	if err != nil {
		cfg, err = clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
			clientcmd.NewDefaultClientConfigLoadingRules(),
			&clientcmd.ConfigOverrides{},
		).ClientConfig()
		if err != nil {
			return nil, nil, err
		}
	}

	client, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, nil, err
	}
	return client, cfg, nil
}
