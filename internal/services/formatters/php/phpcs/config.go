// Copyright 2020 ZUP IT SERVICOS EM TECNOLOGIA E INOVACAO SA
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

package phpcs

// phpcs writes "Time: <x>; Memory: <y>" to stderr, and the analysis container
// merges stderr into stdout, so it would land in front of the JSON and break
// parsing. Keep stderr aside and only surface it when phpcs produced no report,
// so a real failure still reaches the logs.
//
//nolint:all
const CMD = `
		{{WORK_DIR}}
		phpcs --report=json --standard=/vendor/pheromone/phpcs-security-audit/example_drupal7_ruleset.xml . > /tmp/result-ANALYSISID.json 2> /tmp/errorRunning-ANALYSISID
		if [ -s /tmp/result-ANALYSISID.json ]; then
			cat /tmp/result-ANALYSISID.json
		else
			cat /tmp/errorRunning-ANALYSISID
		fi
  	`
