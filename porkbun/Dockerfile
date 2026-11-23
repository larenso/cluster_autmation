FROM alpine:3.24.4

RUN apk add --no-cache ca-certificates

WORKDIR /app
COPY porkbun /app/

ENTRYPOINT ["/app/porkbun"]
