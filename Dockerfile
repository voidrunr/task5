FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/loyalty ./cmd/loyalty \
 && CGO_ENABLED=0 go build -o /out/accrual ./cmd/accrual

FROM alpine:3.20
RUN adduser -D -u 10001 gofer
COPY --from=build /out/loyalty /usr/local/bin/loyalty
COPY --from=build /out/accrual /usr/local/bin/accrual
USER gofer
EXPOSE 8080 8081
ENTRYPOINT ["/usr/local/bin/loyalty"]
