FROM alpine:3.23.4

RUN apk add --no-cache ca-certificates

WORKDIR /app
COPY porkbun /app/

ENTRYPOINT ["/app/porkbun"]
