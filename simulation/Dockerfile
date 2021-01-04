# When building the image, the working directory has to contain dela,
# student20_rabyt and modified go.mod (see comments below)
FROM golang:1.15 as builder
# local version of dela with extra logging, required to evaluate routing performance
COPY ./dela /rabyt/dela
COPY ./student20_rabyt /rabyt/routing
# includes "replace go.dedis.ch/dela => /rabyt/dela"
COPY ./go.mod /rabyt/routing/go.mod
WORKDIR /rabyt/routing/cli/node/memcoin
RUN go build

FROM debian:stretch-slim
RUN apt update && apt install -y procps ca-certificates && apt clean
RUN apt install -y iputils-ping
RUN apt-get install -y net-tools
WORKDIR /root/

COPY --from=builder /rabyt/routing/cli/node/memcoin/memcoin .

ENV LLVL=trace

EXPOSE 2000

CMD ["/root/memcoin", "--config", "/tmp/node", "start", "--port", "2000"]

