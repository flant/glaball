FROM golang:1.21-bullseye as builder

ARG versionflags

WORKDIR /src

COPY . .

RUN CGO_ENABLED=0 go build -v -a -tags netgo -ldflags="-extldflags '-static' -s -w $versionflags" -o build/glaball *.go


FROM debian:bullseye-slim

ENV DEBIAN_FRONTEND=noninteractive

RUN apt-get update && apt-get install -qy --no-install-recommends \
        ca-certificates

COPY --from=builder /src/build/glaball /usr/local/bin/glaball

CMD [ "/usr/local/bin/glaball" ]
