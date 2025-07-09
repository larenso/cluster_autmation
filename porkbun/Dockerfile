FROM alpine:3.22.1

RUN apk add --no-cache ca-certificates

WORKDIR /app
COPY porkbun /app/

ENTRYPOINT ["/app/porkbun"]
