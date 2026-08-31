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

# Python 3.11+ is required by checkov 3.x: below it, checkov depends on
# pyston-autoload, which publishes no distribution for musl, so pip backtracks
# down to checkov 2.1.244. That release does not cap packaging, and packaging
# 22.0 dropped LegacyVersion, so the tool crashed on import.
FROM python:3.12-alpine3.21

RUN apk add --quiet --no-cache wget

RUN wget -O /usr/bin/tfsec https://github.com/aquasecurity/tfsec/releases/download/v0.55.1/tfsec-linux-amd64 && chmod +x /usr/bin/tfsec

# Pinned so the image is reproducible: an unpinned install is what let the
# resolver silently walk back to a 2022 release.
RUN apk add --no-cache --virtual .build_deps build-base libffi-dev \
 && pip install --no-cache-dir checkov==3.3.16 \
 && apk del .build_deps


