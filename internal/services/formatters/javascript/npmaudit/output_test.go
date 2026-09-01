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

package npmaudit

import (
	"encoding/json"
	"testing"

	"github.com/heron-brito/horusec-devkit/pkg/enums/severities"
	"github.com/stretchr/testify/assert"
)

func TestGetVersion(t *testing.T) {
	t.Run("should return finding version", func(t *testing.T) {
		issue := npmIssue{
			Findings: []npmFinding{
				{
					Version: "test",
				},
			},
		}

		assert.Equal(t, "test", issue.getVersion())
	})

	t.Run("should return no version", func(t *testing.T) {
		issue := npmIssue{}
		assert.Empty(t, issue.getVersion())
	})
}

func TestGetSeverity(t *testing.T) {
	t.Run("should return a low severity", func(t *testing.T) {
		issue := npmIssue{
			Severity: "low",
		}

		assert.Equal(t, severities.Low, issue.getSeverity())
	})

	t.Run("should return a medium severity", func(t *testing.T) {
		issue := npmIssue{
			Severity: "moderate",
		}

		assert.Equal(t, severities.Medium, issue.getSeverity())
	})

	t.Run("should return a critical severity", func(t *testing.T) {
		issue := npmIssue{
			Severity: "critical",
		}

		assert.Equal(t, severities.Critical, issue.getSeverity())
	})

	t.Run("should return a info severity", func(t *testing.T) {
		issue := npmIssue{
			Severity: "info",
		}

		assert.Equal(t, severities.Info, issue.getSeverity())
	})

	t.Run("should return a unknown severity", func(t *testing.T) {
		issue := npmIssue{
			Severity: "",
		}

		assert.Equal(t, severities.Unknown, issue.getSeverity())
	})
}

func TestIssues(t *testing.T) {
	t.Run("should read the advisories of an npm 6 report", func(t *testing.T) {
		output := npmOutput{
			Advisories: map[string]npmIssue{
				"1": {ID: 1, ModuleName: "test", Severity: "high"},
			},
		}

		issues := output.issues()

		assert.Len(t, issues, 1)
		assert.Equal(t, "test", issues[0].ModuleName)
		assert.Equal(t, severities.High, issues[0].getSeverity())
	})

	t.Run("should read the vulnerabilities of an npm 7+ report", func(t *testing.T) {
		var output npmOutput
		err := json.Unmarshal([]byte(npmAuditReportVersion2), &output)
		assert.NoError(t, err)

		issues := output.issues()

		assert.Len(t, issues, 1)
		assert.Equal(t, "minimist", issues[0].ModuleName)
		assert.Equal(t, severities.Critical, issues[0].getSeverity())
		assert.Equal(t, "<=0.2.3", issues[0].VulnerableVersions)
		assert.Equal(t, 1096466, issues[0].ID)
		assert.Contains(t, issues[0].Overview, "Prototype Pollution in minimist")
		assert.Contains(t, issues[0].Overview, "https://github.com/advisories/GHSA-vh95-rmgr-6w4m")
	})

	t.Run("should ignore the package names a v2 via array mixes in", func(t *testing.T) {
		var output npmOutput
		err := json.Unmarshal([]byte(npmAuditReportVersion2TransitiveOnly), &output)
		assert.NoError(t, err)

		issues := output.issues()

		assert.Len(t, issues, 1)
		assert.Equal(t, "tough-cookie", issues[0].ModuleName)
		assert.Empty(t, issues[0].Overview)
		assert.Zero(t, issues[0].ID)
	})
}

const npmAuditReportVersion2 = `{
  "auditReportVersion": 2,
  "vulnerabilities": {
    "minimist": {
      "name": "minimist",
      "severity": "critical",
      "isDirect": true,
      "via": [
        {
          "source": 1096466,
          "name": "minimist",
          "title": "Prototype Pollution in minimist",
          "url": "https://github.com/advisories/GHSA-vh95-rmgr-6w4m",
          "severity": "moderate",
          "range": "<0.2.1"
        }
      ],
      "effects": [],
      "range": "<=0.2.3",
      "nodes": ["node_modules/minimist"],
      "fixAvailable": true
    }
  },
  "metadata": {"vulnerabilities": {"info": 0, "low": 0, "moderate": 0, "high": 0, "critical": 1, "total": 1}}
}`

const npmAuditReportVersion2TransitiveOnly = `{
  "auditReportVersion": 2,
  "vulnerabilities": {
    "tough-cookie": {
      "name": "tough-cookie",
      "severity": "moderate",
      "isDirect": false,
      "via": ["request"],
      "effects": ["request"],
      "range": "<4.1.3",
      "nodes": ["node_modules/tough-cookie"],
      "fixAvailable": false
    }
  },
  "metadata": {"vulnerabilities": {"info": 0, "low": 0, "moderate": 1, "high": 0, "critical": 0, "total": 1}}
}`
