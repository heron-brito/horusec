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

FROM --platform=$BUILDPLATFORM golang:1.22-alpine AS builder

# Injected by the publish pipeline so `horusec version` inside the image
# reports the tag the image was published under, instead of the
# {{VERSION_NOT_FOUND}} placeholders compiled into the source.
ARG CLI_VERSION="{{VERSION_NOT_FOUND}}"
ARG CLI_COMMIT="{{COMMIT_NOT_FOUND}}"
ARG CLI_DATE="{{DATE_NOT_FOUND}}"

# Set by buildx for every platform in --platform. The builder stage always
# runs natively (BUILDPLATFORM above) and cross-compiles, so a multi-arch
# build does not pay for emulating the Go toolchain under QEMU.
ARG TARGETARCH

RUN apk add --no-cache git

ADD . /horusec
WORKDIR /horusec

RUN go mod download

RUN env CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH} go build \
    -ldflags "-s -w \
      -X 'github.com/ZupIT/horusec/cmd/app/version.Version=${CLI_VERSION}' \
      -X 'github.com/ZupIT/horusec/cmd/app/version.Commit=${CLI_COMMIT}' \
      -X 'github.com/ZupIT/horusec/cmd/app/version.Date=${CLI_DATE}'" \
    -o /bin/horusec ./cmd/app/main.go

FROM docker:20.10-dind

RUN apk add git --no-cache

COPY --from=builder /bin/horusec /usr/local/bin
RUN chmod +x /usr/local/bin/horusec

CMD [ "sh" ]
