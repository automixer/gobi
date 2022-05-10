FROM golang:1.18-bullseye as builder

WORKDIR /build
COPY . /build

RUN go build -o gobi ./cmd/gobi

FROM ubuntu:22.04

COPY --from=builder /build/gobi /

ENTRYPOINT ["./gobi"]