FROM golang:1.15 as builder
COPY . /rabyt
WORKDIR /rabyt/cli/node/memcoin 
RUN go build

FROM debian:stretch-slim
RUN apt update && apt install -y procps ca-certificates && apt clean
WORKDIR /root/

COPY --from=builder /rabyt/cli/node/memcoin/memcoin .

ENV LLVL=trace

EXPOSE 2000

CMD ["/root/memcoin", "--config", "/tmp/node", "start", "--port", "2000"]

