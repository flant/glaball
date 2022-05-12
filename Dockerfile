FROM golang:1.17-buster as builder

ARG VERSION
ARG REVISION
ARG BRANCH
ARG BUILD_USER
ARG BUILD_DATE

WORKDIR /src

COPY . .

RUN CGO_ENABLED=0 go build -v -a -tags netgo -ldflags="-extldflags '-static' -s -w $versionflags" -o build/gitlaball *.go


FROM debian:buster-slim

RUN DEBIAN_FRONTEND=noninteractive; apt-get update \
    && apt-get install -qy --no-install-recommends \
        ca-certificates \
        tzdata \
        curl

COPY --from=builder /src/build/gitlaball /gitlaball

CMD [ "/gitlaball" ]
