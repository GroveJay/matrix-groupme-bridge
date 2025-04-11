FROM golang:1-alpine3.21 AS go-builder
RUN apk add --no-cache git ca-certificates build-base olm-dev ffmpeg su-exec ca-certificates bash jq curl yq-go

ENV UID=1337 \
    GID=1337

WORKDIR /build
COPY *.go go.* *.yaml *.sh ./
COPY cmd/. cmd/.
COPY pkg/. pkg/.

RUN ./build.sh -o bridge

RUN cp ./bridge /usr/bin/bridge
RUN cp ./docker-run.sh /docker-run.sh
ENV BRIDGEV2=1
VOLUME /data
WORKDIR /data

CMD ["/docker-run.sh"]

