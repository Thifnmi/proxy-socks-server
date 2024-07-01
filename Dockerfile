FROM golang:1.18.3 AS build_base
LABEL maintainer="thifnmi <tuthin2k@gmail.com>"

RUN apt-get update && apt-get install -y git pkg-config

# stage 2
FROM build_base AS build_go

ENV GO111MODULE=on

WORKDIR $GOPATH/proxy-socks-server
COPY go.mod .
COPY go.sum .
RUN go mod download

# stage 3
FROM build_go AS server_builder

ENV GO111MODULE=on

COPY . .
# RUN CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -o /bin/proxy-socks-server ./main.go
RUN go build -o /bin/proxy-socks-server ./main.go

# Stage 4
FROM golang:1.18.3 AS proxy-socks-server

ENV TZ 'Asia/Ho_Chi_Minh'
RUN echo $TZ > /etc/timezone && \
    apt-get update && apt-get install -y tzdata && \
    rm /etc/localtime && \
    ln -snf /usr/share/zoneinfo/$TZ /etc/localtime && \
    dpkg-reconfigure -f noninteractive tzdata && \
    apt-get clean

EXPOSE 8088

COPY --from=server_builder /bin/proxy-socks-server /bin/proxy-socks-server


CMD ["/bin/proxy-socks-server"]


