# Copyright 2021 ZUP IT SERVICOS EM TECNOLOGIA E INOVACAO SA
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

FROM golang:1.22-alpine

RUN apk add curl

# Nancy publishes one binary per architecture, and the amd64 one was hardcoded
# here. On an arm64 build the image assembled fine and only failed at scan
# time, with `exec format error` inside the container.
#
# `TARGETARCH` is set by buildx and already spells the names Nancy uses
# (amd64/arm64). It is empty under a plain `docker build`, which is always a
# native build, so `uname -m` is the right answer there.
ARG TARGETARCH
RUN ARCH="${TARGETARCH:-$(uname -m | sed 's/x86_64/amd64/; s/aarch64/arm64/')}" \
 && curl -fsSL "https://github.com/sonatype-nexus-community/nancy/releases/download/v1.0.33/nancy-v1.0.33-linux-${ARCH}" -o /bin/nancy \
 && chmod +x /bin/nancy

RUN wget -O - -q https://raw.githubusercontent.com/securego/gosec/master/install.sh | sh -s v2.22.11
