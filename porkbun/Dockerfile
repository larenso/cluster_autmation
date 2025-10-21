FROM alpine:3.22.2

RUN apk add --no-cache ca-certificates

WORKDIR /app
COPY porkbun /app/

ENTRYPOINT ["/app/porkbun"]
