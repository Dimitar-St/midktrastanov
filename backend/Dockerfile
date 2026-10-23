FROM golang:1.25-alpine AS build
WORKDIR /app
COPY go.mod go.sum ./
COPY *.go ./
COPY migrations ./migrations
RUN CGO_ENABLED=0 go build -o /server .
FROM alpine:3.22
RUN apk add --no-cache ca-certificates && adduser -D -u 10001 app
USER app
COPY --from=build /server /server
ENV PORT=8080
EXPOSE 8080
ENTRYPOINT ["/server"]
