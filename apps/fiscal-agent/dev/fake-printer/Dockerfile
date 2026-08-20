FROM golang:1.22-alpine AS build
WORKDIR /src
COPY go.mod main.go ./
RUN go build -o /fake-printer .

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
COPY --from=build /fake-printer /usr/local/bin/fake-printer
ENV PRINTER_NAME=fake
ENV PRINTER_PORT=9100
EXPOSE 9100
ENTRYPOINT ["fake-printer"]
