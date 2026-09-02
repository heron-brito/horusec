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

# The SDK only builds target frameworks up to its own major, so SDK 6 rejected
# every project on .NET 7 or later with NETSDK1045, and dotnet list package
# --vulnerable then failed for want of an assets file. Those projects were
# reported clean rather than unscanned. SDK 10 also restores net6.0 and net8.0,
# so older projects keep working.
FROM mcr.microsoft.com/dotnet/sdk:10.0-alpine

RUN apk add --no-cache jq

RUN dotnet tool install --global security-scan --version 5.6.7

# security-scan targets .NET 6, which this image does not carry, so without
# rolling forward it installs and then refuses to start: "You must install or
# update .NET to run this application."
ENV DOTNET_ROLL_FORWARD=Major

# The analysis container runs with a TTY, and under one the .NET CLI writes
# terminal setup sequences (ESC [ ? 1 h ESC =) straight to the terminal device.
# They land in front of the SARIF the command cats out and break the parse with
# "invalid character '\x1b'". Redirecting cannot catch them because they do not
# go through stdout or stderr; declaring no terminal capabilities stops them.
ENV TERM=dumb

ENV PATH="$PATH:/root/.dotnet/tools"
