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

# Alpine 3.16 carries shellcheck for x86_64 only - no aarch64 build exists in
# its repositories -, e o 3.16 está EOL desde 2024. O 3.21 é o mesmo já usado
# pelas imagens de python e hcl e traz shellcheck para as duas arquiteturas.
#
# A ferramenta sobe de 0.8.0 para 0.10.0 junto: é um scanner, então achados
# novos são esperados.
FROM alpine:3.21

# As aspas são o conserto de um bug silencioso: sem elas o `>` é redirecionamento
# de shell, não restrição de versão. O comando instalava shellcheck em qualquer
# versão e gravava um arquivo `/0.7.2` na imagem.
RUN apk add --no-cache 'shellcheck>0.7.2'
