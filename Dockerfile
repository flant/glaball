FROM golang:1.17-buster as builder

ARG versionflags

WORKDIR /src

COPY . .

RUN CGO_ENABLED=0 go build -v -a -tags netgo -ldflags="-extldflags '-static' -s -w $versionflags" -o build/gitlaball *.go


FROM debian:buster-slim

ENV DEBIAN_FRONTEND=noninteractive

RUN apt-get update && apt-get install -qy --no-install-recommends \
        ca-certificates \
        bash-completion

COPY --from=builder /src/build/gitlaball /usr/local/bin/gitlaball

RUN /usr/local/bin/gitlaball completion bash >> /etc/bash_completion

CMD [ "/usr/local/bin/gitlaball" ]
