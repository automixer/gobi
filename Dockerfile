FROM golang:1.18-bullseye as builder

WORKDIR /app
COPY . /app

RUN make clean release

FROM ubuntu:22.04

COPY --from=builder /app/build/* /

ENTRYPOINT ["./gobi"]