FROM golang:1.26-alpine

ARG DOCKER_PATH
ARG WORKDIR
ARG CONFIG_PATH
ARG ROOT_PATH

ENV WORKDIR=${WORKDIR}
ENV CONFIG_PATH=${CONFIG_PATH}
ENV ROOT_PATH=${ROOT_PATH}

ENV HOME=/home

ENV GOCACHE=/go/cache/go-build
ENV GOPRIVATE=github.com/precision-soft
ENV GONOSUMDB=github.com/precision-soft

RUN apk add --update --no-cache git shadow bash gcc musl-dev

ARG STATICCHECK_VERSION=v0.8.1
RUN go install honnef.co/go/tools/cmd/staticcheck@${STATICCHECK_VERSION}

ARG GOVULNCHECK_VERSION=v1.7.0
RUN go install golang.org/x/vuln/cmd/govulncheck@${GOVULNCHECK_VERSION}

RUN mkdir -p "${HOME}"
RUN mkdir -p /go/cache/go-build

COPY ${DOCKER_PATH}/.profile* ${HOME}/

ENV ENV="${HOME}/.profile"

RUN printf '%s\n' \
  'if [ -f "$HOME/.profile" ]; then' \
  '  . "$HOME/.profile"' \
  'fi' \
  > "${HOME}/.bashrc" \
  && printf '%s\n' \
  'if [ -f "$HOME/.bashrc" ]; then' \
  '  . "$HOME/.bashrc"' \
  'fi' \
  > "${HOME}/.bash_profile"

WORKDIR ${WORKDIR}

COPY ${DOCKER_PATH}/entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

ENTRYPOINT ["/entrypoint.sh"]
