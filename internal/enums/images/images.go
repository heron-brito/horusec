// Copyright 2021 ZUP IT SERVICOS EM TECNOLOGIA E INOVACAO SA
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

package images

import "github.com/heron-brito/horusec-devkit/pkg/enums/languages"

const (
	DefaultRegistry = "ghcr.io"
	C               = "heron-brito/horusec-c:v2.10.2"
	Csharp          = "heron-brito/horusec-csharp:v2.10.2"
	Elixir          = "heron-brito/horusec-elixir:v2.10.2"
	Generic         = "heron-brito/horusec-generic:v2.10.2"
	Go              = "heron-brito/horusec-go:v2.10.2"
	HCL             = "heron-brito/horusec-hcl:v2.10.2"
	Javascript      = "heron-brito/horusec-js:v2.10.2"
	Leaks           = "heron-brito/horusec-leaks:v2.10.2"
	PHP             = "heron-brito/horusec-php:v2.10.2"
	Python          = "heron-brito/horusec-python:v2.10.2"
	Ruby            = "heron-brito/horusec-ruby:v2.10.2"
	Shell           = "heron-brito/horusec-shell:v2.10.2"
)

func MapValues() map[languages.Language]string {
	return map[languages.Language]string{
		languages.CSharp:     Csharp,
		languages.Leaks:      Leaks,
		languages.Go:         Go,
		languages.Javascript: Javascript,
		languages.Python:     Python,
		languages.Ruby:       Ruby,
		languages.HCL:        HCL,
		languages.Generic:    Generic,
		languages.PHP:        PHP,
		languages.Elixir:     Elixir,
		languages.Shell:      Shell,
		languages.C:          C,
	}
}
