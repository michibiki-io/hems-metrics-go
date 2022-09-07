#####################################
############# For Build #############
#####################################
FROM golang:latest as builder

RUN apt-get update && \
    apt-get autoremove -y && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/*

RUN mkdir -p /opt/go

WORKDIR /opt/go

COPY . .

# build
RUN go mod tidy && go build -o ./metrics ./main.go

CMD ["/bin/bash"]

# #################################
# ########## For RUNTIME ##########
# #################################
FROM debian:bullseye-slim as runtime

RUN apt-get update && \
    apt-get autoremove -y && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/*

RUN mkdir -p /opt/go

COPY --from=builder /opt/go/metrics /opt/go/metrics

RUN groupadd go && \
    useradd -m -s /bin/bash -g go go && \
    chown go:go /opt/go -R

# Envs
ENV GIN_MODE=release \
    B_ROUTE_ID="0123456789AB" \
    B_ROUTE_PASSWORD="0123456789ABCDEF0123456789ABCDEF" \
    CONNECT_RETRY_COUNT="5" \
    REFRESH_SECONDS="5" \
    POWER_CONSUMPTION_CRON_EXPR_STRING="0,30 * * * *"

WORKDIR /opt/go

USER go

VOLUME ["/dev/ttyUSB0"]

EXPOSE 9090

CMD ["/opt/go/metrics"]
