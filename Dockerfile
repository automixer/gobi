FROM golang:1.25.4 AS builder

WORKDIR /app
COPY . /app

RUN make clean release

FROM ubuntu:24.04

COPY --from=builder /app/build/* /

ENTRYPOINT ["./gobi"]