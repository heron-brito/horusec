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

package nancy

// Sonatype OSS Index no longer serves anonymous requests, so nancy needs
// credentials to reach it. They are optional here to keep the tool usable for
// whoever already has a working setup: without them nancy runs as before.
const CMD = `
		{{WORK_DIR}}
		if [ -n "$OSSINDEX_USERNAME" ] && [ -n "$OSSINDEX_TOKEN" ]; then
			go list -json -m all | nancy sleuth -o json --username "$OSSINDEX_USERNAME" --token "$OSSINDEX_TOKEN"
		else
			go list -json -m all | nancy sleuth -o json
		fi
	`
