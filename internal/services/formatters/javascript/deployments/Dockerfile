# Copyright 2020 ZUP IT SERVICOS EM TECNOLOGIA E INOVACAO SA
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# npm 6 cannot read lockfileVersion 2 or 3, the format npm 7+ writes, and fails
# with "Cannot read properties of undefined" instead of auditing. Every modern
# project was silently getting no npm audit coverage at all. Node 24 ships npm
# 11, which reads every lockfile version; its report is auditReportVersion 2,
# handled by the npmaudit formatter.
FROM node:24-alpine

# yarn audit output is reshaped with jq. Taking it from the alpine repository
# instead of the 2015 amd64-only release drops an architecture lock-in.
RUN apk add --no-cache jq
