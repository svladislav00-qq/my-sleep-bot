FROM golang:1.26.2-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /sleep-bot .

FROM alpine:3.22

WORKDIR /app
COPY --from=builder /sleep-bot ./sleep-bot

CMD [ "./sleep-bot" ]